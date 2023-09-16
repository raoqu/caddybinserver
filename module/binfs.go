package module

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

type BinFS struct {
	Inited bool

	Contents map[string][]byte
	Paths    map[string]int
}

type BinFSFile struct {
	FilePath string
	Bytes    []byte
	Offset   int64
	FileSize int64
	Fs       *BinFS
}

func (binfs *BinFS) innerOpen(path string) (*BinFSFile, error) {
	bytes, ok := binfs.Contents[path]
	if ok {
		file := BinFSFile{FilePath: path, Bytes: bytes, Offset: 0, FileSize: int64(len(bytes)), Fs: binfs}
		return &file, nil
	}

	_, dir := binfs.Paths[path]
	if dir {
		file := BinFSFile{FilePath: path, Bytes: nil, Offset: 0, FileSize: 0, Fs: binfs}
		return &file, nil
	}

	return nil, errors.New("unable to open")
}

func (binfs *BinFS) Open(path string) (fs.File, error) {
	file, err := binfs.innerOpen(path)
	return file, err
}

func (binfs *BinFS) Stat(path string) (fs.FileInfo, error) {
	file, err := binfs.innerOpen(path)
	return file, err
}
func (binfs *BinFS) Glob(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}
func (binfs *BinFS) ReadDir(name string) ([]fs.DirEntry, error) {
	panic("ReadDir")
}
func (binfs *BinFS) ReadFile(path string) ([]byte, error) {
	file, err := binfs.innerOpen(path)
	if err != nil {
		return nil, err
	}
	return file.Bytes, nil
}

func (file *BinFSFile) Open() (fs.FileInfo, error) {
	return file, nil
}

func (file *BinFSFile) Stat() (fs.FileInfo, error) {
	return file, nil
}
func (file *BinFSFile) Read(p []byte) (int, error) {
	n, err := bytes.NewBuffer(file.Bytes[file.Offset:]).Read(p)

	if err == nil {
		if file.Offset+int64(len(p)) < file.FileSize {
			file.Offset += int64(len(p))
		} else {
			file.Offset = int64(file.FileSize)
		}
	}

	return n, err
}
func min_int64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func (file *BinFSFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		file.Offset = min_int64(file.FileSize, offset)
		break
	case io.SeekCurrent:
		file.Offset = min_int64(file.FileSize, file.Offset+offset)
		break
	case io.SeekEnd:
		file.Offset = min_int64(file.FileSize, file.FileSize+offset)
		break
	}
	return file.Offset, nil
}
func (file *BinFSFile) Close() error { return nil }

func (file *BinFSFile) Name() string { return file.FilePath }

func (file *BinFSFile) Size() int64 {
	if file.Bytes == nil {
		return 0
	}
	return int64(len(file.Bytes))
}

func (file *BinFSFile) Mode() fs.FileMode {
	if file.Bytes == nil {
		return fs.ModeDir
	}
	return 0
}
func (file *BinFSFile) ModTime() time.Time { return time.UnixMilli(1672502400000) }
func (file *BinFSFile) IsDir() bool        { return file.Bytes == nil }
func (file *BinFSFile) Sys() any           { return nil }

const DEFAULT_BINFS_FILENAME = "data.bin"

func (fs *BinFS) initResource() error {
	// start server if necessary
	appName := "server"
	if runtime.GOOS == "windows" {
		appName = "server.exe"
	}
	if fileExists(appName) {
		startCommand(appName, ".", true)
	}
	// close server before exiting
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signals
		time.Sleep(500 * time.Millisecond)
		terminateAllCommands()
		os.Exit(1)
	}()

	if fs.Inited {
		return nil
	}
	fs.Inited = true
	fs.Contents = make(map[string][]byte)
	fs.Paths = make(map[string]int)

	data, err := os.ReadFile(DEFAULT_BINFS_FILENAME)
	// verify format: file start with 'DATA'
	if err != nil {
		return fmt.Errorf("resource not found: %s", DEFAULT_BINFS_FILENAME)
	}

	zipData := bytes.NewReader(data)
	r, err := zip.NewReader(zipData, int64(len(data)))
	if err != nil {
		return fmt.Errorf("failed load packed resources")
	}
	for _, f := range r.File {

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed open zipped file: %s", f.Name)
		}
		defer rc.Close()

		// load zipped file contents
		fdata, err := io.ReadAll(rc)
		if err != nil {
			return fmt.Errorf("cannot read zipped content: %s", f.Name)
		}

		fs.addFile(f.Name, fdata)
		fmt.Println(f.Name)
	}

	return nil
}

func (fs *BinFS) addFile(filePath string, content []byte) {
	filePath = strings.Replace(filePath, "/", "\\", -1)

	fs.Contents[filePath] = content
	path := filepath.Dir(filePath)
	if len(path) > 0 {
		fs.Paths[path] = 1
	}
}

func (fs *BinFS) hasPath(path string) bool {
	_, ok := fs.Paths[path]
	return ok
}
