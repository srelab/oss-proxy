package sftp

// This serves as an example of how to implement the request server handler as
// well as a dummy backend for testing. It implements an in-memory backend that
// works as a very simple filesystem with simple flat key-value lookup system.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/denverdino/aliyungo/oss"
	"github.com/pkg/errors"
	"github.com/pkg/sftp"
)

var Client *oss.Client
var Bucket *oss.Bucket
var FileSystem *filesystem

type FTime time.Time

func (t FTime) MarshalJSON() ([]byte, error) {
	ts := fmt.Sprintf("\"%s\"", time.Time(t).Add(time.Hour*8).Format("2006-01-02 15:04:05"))
	return []byte(ts), nil
}

// Implements os.FileInfo, Reader and Writer interfaces.
// These are the 3 interfaces necessary for the Handlers.
type memFile struct {
	Fname       string `json:"name"`
	Modtime     FTime  `json:"modtime"`
	Symlink     string `json:"symlink,omitempty"`
	Isdir       bool   `json:"isdir"`
	Fsize       int64  `json:"size"`
	URL         string `json:"url,omitempty"`
	Hide        bool   `json:"hide"`
	content     []byte
	contentLock sync.RWMutex
}

// In memory file-system-y thing that the Hanlders live on
type filesystem struct {
	*memFile
	files     map[string]*memFile
	filesLock sync.Mutex
	mockErr   error
}

func InitFileSystem() {
	Client = oss.NewOSSClient(
		"",
		false,
		"",
		"",
		false,
	)

	Bucket = Client.Bucket("welab-ftp")

	FileSystem = &filesystem{
		files: make(map[string]*memFile),
	}

	FileSystem.memFile = newMemFile("/", true, true, 0, time.Now())
}

// NewHandler returns a Hanlders object with the test handlers.
func NewOssHandler() sftp.Handlers {
	return sftp.Handlers{FileGet: FileSystem, FilePut: FileSystem, FileCmd: FileSystem, FileList: FileSystem}
}

// Example Handlers
func (fs *filesystem) Fileread(r *sftp.Request) (io.ReaderAt, error) {
	if fs.mockErr != nil {
		return nil, fs.mockErr
	}

	fs.filesLock.Lock()
	defer fs.filesLock.Unlock()

	file, err := fs.fetch(r.Filepath)
	if err != nil {
		return nil, err
	}

	if file.Symlink != "" {
		file, err = fs.fetch(file.Symlink)
		if err != nil {
			return nil, err
		}
	}

	content, err := Bucket.Get(file.OssPath(r.Filepath))
	if err != nil {
		return nil, err
	}

	return file.ReaderAt(content)
}

func (fs *filesystem) Filewrite(r *sftp.Request) (io.WriterAt, error) {
	if fs.mockErr != nil {
		return nil, fs.mockErr
	}

	fs.filesLock.Lock()
	defer fs.filesLock.Unlock()

	file, err := fs.fetch(r.Filepath)
	if err == os.ErrNotExist {
		dir, err := fs.fetch(filepath.Dir(r.Filepath))
		if err != nil {
			return nil, err
		}

		if !dir.Isdir {
			return nil, os.ErrInvalid
		}

		file = newMemFile(r.Filepath, false, false, 0, time.Now())
		fs.files[r.Filepath] = file
	}

	return file.WriterAt()
}

func (fs *filesystem) Filecmd(r *sftp.Request) error {
	if fs.mockErr != nil {
		return fs.mockErr
	}

	fs.filesLock.Lock()
	defer fs.filesLock.Unlock()

	// Update the OSS file list with the requested file path
	if files, err := fs.FetchFiles(filepath.Dir(r.Filepath), false); err == nil {
		fs.files = files
	} else {
		return err
	}

	switch r.Method {
	case "Setstat":
		return nil
	case "Rename":
		file, err := fs.fetch(r.Filepath)
		if err != nil {
			return err
		}

		if _, ok := fs.files[r.Target]; ok {
			return &os.LinkError{Op: "rename", Old: r.Filepath, New: r.Target,
				Err: fmt.Errorf("dest file exists")}
		}

		// Copy
		target := file.OssPath(r.Target)
		source := Bucket.Path(file.OssPath(r.Filepath))
		if _, err := Bucket.PutCopy(target, oss.Private, oss.CopyOptions{}, source); err != nil {
			return err
		}

		if err := Bucket.Del(file.OssPath(r.Filepath)); err != nil {
			return err
		}

		file.Fname = filepath.Base(r.Target)
		fs.files[r.Target] = file

		delete(fs.files, r.Filepath)
	case "Rmdir", "Remove":
		file, err := fs.fetch(r.Filepath)
		if err != nil {
			return err
		}

		if err := Bucket.Del(file.OssPath(r.Filepath)); err != nil {
			return err
		}

		delete(fs.files, r.Filepath)
	case "Mkdir":
		_, err := fs.fetch(filepath.Dir(r.Filepath))
		if err != nil {
			return err
		}

		dirPath := strings.TrimLeft(r.Filepath, "/") + "/"
		if err := Bucket.Put(dirPath, []byte{}, "content-type", oss.Private, oss.Options{}); err != nil {
			return err
		}

		fs.files[r.Filepath] = newMemFile(filepath.Base(r.Filepath), true, false, 0, time.Now())
	case "Symlink":
		return errors.New("Protocol error.")
	}

	return nil
}

type listerat []os.FileInfo

// Modeled after strings.Reader's ReadAt() implementation
func (f listerat) ListAt(ls []os.FileInfo, offset int64) (int, error) {
	var n int
	if offset >= int64(len(f)) {
		return 0, io.EOF
	}

	n = copy(ls, f[offset:])
	if n < len(ls) {
		return n, io.EOF
	}

	return n, nil
}

func (fs *filesystem) FetchFiles(prefix string, recursive bool) (files map[string]*memFile, err error) {
	files = make(map[string]*memFile, 0)
	prefix = strings.TrimLeft(prefix+"/", "/")

	delim := "/"
	if recursive {
		delim = ""
	}

	br, err := Bucket.List(prefix, delim, "", 1000)
	if err != nil {
		return files, fmt.Errorf("unable to get list of oss files: %s", err)
	}

	for _, content := range br.Contents {
		modtime, err := time.Parse(time.RFC3339, content.LastModified)
		if err != nil {
			modtime = time.Now()
		}

		path := filepath.Join("/", strings.TrimRight(content.Key, "/"))
		fn := filepath.Base(path)

		if strings.HasSuffix(content.Key, "/") {
			files[path] = newMemFile(fn, true, true, content.Size, modtime)
		} else {
			files[path] = newMemFile(fn, false, false, content.Size, modtime)
		}

		files[path].Fsize = content.Size
	}

	for _, commonPrefix := range br.CommonPrefixes {
		path := filepath.Join("/", strings.TrimRight(commonPrefix, "/"))
		fn := filepath.Base(path)

		files[path] = newMemFile(fn, true, false, 0, time.Now())
	}

	return files, err
}

func (fs *filesystem) Filelist(r *sftp.Request) (sftp.ListerAt, error) {
	if fs.mockErr != nil {
		return nil, fs.mockErr
	}

	fs.filesLock.Lock()
	defer fs.filesLock.Unlock()

	switch r.Method {
	case "List":
		// Update the OSS file list with the requested file path
		if files, err := fs.FetchFiles(r.Filepath, false); err == nil {
			fs.files = files
		} else {
			return nil, err
		}

		var paths []string
		for fp := range fs.files {
			if filepath.Dir(fp) == r.Filepath {
				paths = append(paths, fp)
			}
		}

		sort.Strings(paths)
		files := make([]os.FileInfo, len(paths))

		for index, fp := range paths {
			files[index] = fs.files[fp]
		}

		return listerat(files), nil
	case "Stat":
		// Update the OSS file list with the requested file path
		if files, err := fs.FetchFiles(filepath.Dir(r.Filepath), false); err == nil {
			fs.files = files
		} else {
			return nil, err
		}

		file, err := fs.fetch(r.Filepath)
		if err != nil {
			return nil, err
		}

		return listerat([]os.FileInfo{file}), nil
	case "Readlink":
		// Update the OSS file list with the requested file path
		if files, err := fs.FetchFiles(filepath.Dir(r.Filepath), false); err == nil {
			fs.files = files
		} else {
			return nil, err
		}

		file, err := fs.fetch(r.Filepath)
		if err != nil {
			return nil, err
		}

		if file.Symlink != "" {
			file, err = fs.fetch(file.Symlink)
			if err != nil {
				return nil, err
			}
		}

		return listerat([]os.FileInfo{file}), nil
	}
	return nil, nil
}

// Set a mocked error that the next handler call will return.
// Set to nil to reset for no error.
func (fs *filesystem) returnErr(err error) {
	fs.mockErr = err
}

func (fs *filesystem) fetch(path string) (*memFile, error) {
	if path == "/" {
		return fs.memFile, nil
	}

	if file, ok := fs.files[path]; ok {
		return file, nil
	}

	return nil, os.ErrNotExist
}

// factory to make sure modtime is set
func newMemFile(name string, isdir bool, hide bool, size int64, modtime time.Time) *memFile {
	return &memFile{
		Fname:   name,
		Modtime: FTime(modtime),
		Isdir:   isdir,
		Hide:    hide,
		Fsize:   size,
	}
}

// Have memFile fulfill os.FileInfo interface
func (f *memFile) Name() string { return filepath.Base(f.Fname) }
func (f *memFile) Size() int64  { return f.Fsize }
func (f *memFile) Mode() os.FileMode {
	ret := os.FileMode(0644)
	if f.Isdir {
		ret = os.FileMode(0755) | os.ModeDir
	}
	if f.Symlink != "" {
		ret = os.FileMode(0777) | os.ModeSymlink
	}
	return ret
}

func (f *memFile) ModTime() time.Time { return time.Time(f.Modtime) }
func (f *memFile) IsDir() bool        { return f.Isdir }
func (f *memFile) Sys() interface{}   { return &syscall.Stat_t{Uid: 65534, Gid: 65534} }

func (f *memFile) OssPath(path string) string {
	if f.Isdir {
		return strings.TrimLeft(path, "/") + "/"
	}

	return strings.TrimLeft(path, "/")
}

// Read/Write
func (f *memFile) ReaderAt(content []byte) (io.ReaderAt, error) {
	if f.Isdir {
		return nil, os.ErrInvalid
	}
	return bytes.NewReader(content), nil
}

func (f *memFile) WriterAt() (io.WriterAt, error) {
	if f.Isdir {
		return nil, os.ErrInvalid
	}

	return f, nil
}

func (f *memFile) WriteAt(p []byte, off int64) (int, error) {
	f.contentLock.Lock()
	defer f.contentLock.Unlock()

	plen := len(p) + int(off)
	if plen >= len(f.content) {
		nc := make([]byte, plen)
		copy(nc, f.content)
		f.content = nc
	}

	copy(f.content[off:], p)

	Bucket.Put(f.Fname, f.content, "content-type", oss.Private, oss.Options{})
	return len(p), nil
}
