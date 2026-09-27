package http

import (
	"fmt"
	"io"
	"net/http"
)

// multipartFileField streams the named file part without buffering the whole upload in RAM.
func multipartFileField(r *http.Request, fieldName string) (file io.ReadCloser, err error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, fmt.Errorf("multipart reader: %w", err)
	}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			return nil, fmt.Errorf("field %q not found", fieldName)
		}
		if err != nil {
			return nil, err
		}
		if part.FormName() != fieldName {
			part.Close()
			continue
		}
		return part, nil
	}
}
