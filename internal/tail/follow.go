package tail

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Follow continuously reads new content from a file and writes it to the given writer.
// It polls the file for changes and streams new content as it appears.
// This function blocks until an error occurs or the context is cancelled.
func Follow(filePath string, w io.Writer) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Start from the beginning of the file
	offset := int64(0)

	buf := make([]byte, 4096)

	for {
		// Seek to current position
		_, err := file.Seek(offset, io.SeekStart)
		if err != nil {
			return err
		}

		// Read available data
		n, err := file.Read(buf)
		if n > 0 {
			_, writeErr := w.Write(buf[:n])
			if writeErr != nil {
				return writeErr
			}
			offset += int64(n)
		}

		if err != nil && err != io.EOF {
			return err
		}

		// If we got EOF or no data, wait a bit before polling again
		if n == 0 || err == io.EOF {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// FileSource represents a file to follow with an optional prefix for each line
type FileSource struct {
	Path   string
	Prefix string
}

// Follower manages following multiple files with support for dynamic source addition
type Follower struct {
	w       io.Writer
	mu      sync.Mutex
	sources map[string]bool // tracks which paths are already being followed
	errCh   chan error
	wg      sync.WaitGroup
	done    chan struct{}
	stopped bool
}

// SystemLogTag is the prefix used for system log messages (same length as job IDs)
const SystemLogTag = "gob"

// SystemLog writes a system log message with the monitor prefix
// The message is colored cyan to distinguish it from job output
func (f *Follower) SystemLog(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	// Cyan color for system messages
	prefix := fmt.Sprintf("\033[36m[%s]\033[0m ", SystemLogTag)
	f.mu.Lock()
	f.w.Write([]byte(prefix + msg + "\n"))
	f.mu.Unlock()
}

// NewFollower creates a new Follower that writes to the given writer
func NewFollower(w io.Writer) *Follower {
	return &Follower{
		w:       w,
		sources: make(map[string]bool),
		errCh:   make(chan error, 100),
		done:    make(chan struct{}),
	}
}

// AddSource adds a new file source to follow. If the source is already being
// followed, this is a no-op.
func (f *Follower) AddSource(source FileSource) {
	f.mu.Lock()
	if f.sources[source.Path] {
		f.mu.Unlock()
		return
	}
	f.sources[source.Path] = true
	f.mu.Unlock()

	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		err := followWithPrefix(source.Path, source.Prefix, f.w, &f.mu, f.done)
		if err != nil {
			f.errCh <- err
		}
	}()
}

// Stop signals all followers to stop and waits for them to finish
func (f *Follower) Stop() {
	f.mu.Lock()
	if f.stopped {
		f.mu.Unlock()
		return
	}
	f.stopped = true
	f.mu.Unlock()
	close(f.done)
	f.wg.Wait()
}

// Wait blocks until an error occurs from any source or the follower is stopped
func (f *Follower) Wait() error {
	select {
	case err := <-f.errCh:
		return err
	case <-f.done:
		return nil
	}
}

// FollowMultiple continuously reads from multiple files and writes to the given writer.
// Lines from files with a prefix set will have that prefix prepended.
// This function blocks until an error occurs.
func FollowMultiple(sources []FileSource, w io.Writer) error {
	f := NewFollower(w)
	for _, src := range sources {
		f.AddSource(src)
	}
	return f.Wait()
}

// followWithPrefix follows a file and prefixes each line with the given prefix
func followWithPrefix(filePath string, prefix string, w io.Writer, mu *sync.Mutex, done <-chan struct{}) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	offset := int64(0)
	buf := make([]byte, 4096)
	var lineBuf bytes.Buffer

	for {
		// Check if we should stop
		select {
		case <-done:
			return nil
		default:
		}

		_, err := file.Seek(offset, io.SeekStart)
		if err != nil {
			return err
		}

		n, err := file.Read(buf)
		if n > 0 {
			offset += int64(n)

			// Process the buffer to add prefixes to complete lines
			data := buf[:n]
			for len(data) > 0 {
				idx := bytes.IndexByte(data, '\n')
				if idx >= 0 {
					// Found a newline - write the complete line with prefix
					lineBuf.Write(data[:idx+1])
					line := lineBuf.Bytes()
					lineBuf.Reset()

					mu.Lock()
					if prefix != "" {
						w.Write([]byte(prefix))
					}
					w.Write(line)
					mu.Unlock()

					data = data[idx+1:]
				} else {
					// No newline - buffer the data for the next read
					lineBuf.Write(data)
					break
				}
			}
		}

		if err != nil && err != io.EOF {
			return err
		}

		if n == 0 || err == io.EOF {
			time.Sleep(100 * time.Millisecond)
		}
	}
}
