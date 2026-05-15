package messagelog

import (
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

var sensitiveHeaders = map[string]struct{}{
	"authorization":   {},
	"x-api-key":       {},
	"api-key":         {},
	"x-goog-api-key":  {},
	"x-secret-key":    {},
	"x-auth-token":    {},
	"proxy-authorization": {},
}

func SanitizeHeaders(h http.Header) map[string]string {
	result := make(map[string]string, len(h))
	for k, v := range h {
		if _, sensitive := sensitiveHeaders[strings.ToLower(k)]; sensitive {
			continue
		}
		result[k] = strings.Join(v, ", ")
	}
	return result
}

func FlattenHeaders(h http.Header) map[string]string {
	result := make(map[string]string, len(h))
	for k, v := range h {
		result[k] = strings.Join(v, ", ")
	}
	return result
}

func MarshalHeaders(h map[string]string) string {
	if len(h) == 0 {
		return ""
	}
	data, err := common.Marshal(h)
	if err != nil {
		return ""
	}
	return string(data)
}

type ReadCloserWrapper struct {
	reader  io.Reader
	closer  io.Closer
	onClose func()
	once    sync.Once
}

func NewReadCloserWrapper(reader io.Reader, closer io.Closer, onClose func()) *ReadCloserWrapper {
	return &ReadCloserWrapper{
		reader:  reader,
		closer:  closer,
		onClose: onClose,
	}
}

func (r *ReadCloserWrapper) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

func (r *ReadCloserWrapper) Close() error {
	err := r.closer.Close()
	r.once.Do(func() {
		if r.onClose != nil {
			r.onClose()
		}
	})
	return err
}
