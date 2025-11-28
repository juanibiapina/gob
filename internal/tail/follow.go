package tail

import (
	"io"
	"os"
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
