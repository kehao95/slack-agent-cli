package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type eventLineSink interface {
	WriteLine(line []byte) error
}

type writerLineSink struct {
	writer io.Writer
}

func (s writerLineSink) WriteLine(line []byte) error {
	if _, err := s.writer.Write(line); err != nil {
		return err
	}
	_, err := s.writer.Write([]byte("\n"))
	return err
}

type fileAppenderOpener interface {
	Open(name string, flag int, perm os.FileMode) (io.WriteCloser, error)
}

type osFileAppenderOpener struct{}

func (osFileAppenderOpener) Open(name string, flag int, perm os.FileMode) (io.WriteCloser, error) {
	return os.OpenFile(name, flag, perm)
}

type appendFileLineSink struct {
	path   string
	opener fileAppenderOpener
}

func newAppendFileLineSink(path string) appendFileLineSink {
	return appendFileLineSink{
		path:   path,
		opener: osFileAppenderOpener{},
	}
}

func (s appendFileLineSink) WriteLine(line []byte) error {
	file, err := s.opener.Open(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", s.path, err)
	}

	writeErr := writerLineSink{writer: file}.WriteLine(line)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return fmt.Errorf("close %s: %w", s.path, closeErr)
	}
	return nil
}

type multiEventLineSink struct {
	sinks []eventLineSink
}

func (s multiEventLineSink) WriteLine(line []byte) error {
	for _, sink := range s.sinks {
		if err := sink.WriteLine(line); err != nil {
			return err
		}
	}
	return nil
}

func newEventsStreamSink(cmd *cobra.Command) (eventLineSink, error) {
	sinks := []eventLineSink{
		writerLineSink{writer: cmd.OutOrStdout()},
	}

	filePath, _ := cmd.Flags().GetString("file")
	filePath = strings.TrimSpace(filePath)
	if filePath != "" {
		sinks = append(sinks, newAppendFileLineSink(filePath))
	}

	return multiEventLineSink{sinks: sinks}, nil
}

func formatStreamEventLine(event streamEvent, human bool) ([]byte, error) {
	if human {
		return []byte(formatHumanStreamEvent(event)), nil
	}

	line, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("encode event: %w", err)
	}
	return line, nil
}
