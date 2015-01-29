/*
 * Copyright (c) 2015, Psiphon Inc.
 * All rights reserved.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package psiphon

import (
	"bytes"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"runtime"
	"strings"
	"sync"
)

// Contains is a helper function that returns true
// if the target string is in the list.
func Contains(list []string, target string) bool {
	for _, listItem := range list {
		if listItem == target {
			return true
		}
	}
	return false
}

// MakeSecureRandomInt is a helper function that wraps
// MakeSecureRandomInt64.
func MakeSecureRandomInt(max int) (int, error) {
	randomInt, err := MakeSecureRandomInt64(int64(max))
	return int(randomInt), err
}

// MakeSecureRandomInt64 is a helper function that wraps
// crypto/rand.Int, which returns a uniform random value in [0, max).
func MakeSecureRandomInt64(max int64) (int64, error) {
	randomInt, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		return 0, ContextError(err)
	}
	return randomInt.Int64(), nil
}

// MakeSecureRandomBytes is a helper function that wraps
// crypto/rand.Read.
func MakeSecureRandomBytes(length int) ([]byte, error) {
	randomBytes := make([]byte, length)
	n, err := rand.Read(randomBytes)
	if err != nil {
		return nil, ContextError(err)
	}
	if n != length {
		return nil, ContextError(errors.New("insufficient random bytes"))
	}
	return randomBytes, nil
}

func DecodeCertificate(encodedCertificate string) (certificate *x509.Certificate, err error) {
	derEncodedCertificate, err := base64.StdEncoding.DecodeString(encodedCertificate)
	if err != nil {
		return nil, ContextError(err)
	}
	certificate, err = x509.ParseCertificate(derEncodedCertificate)
	if err != nil {
		return nil, ContextError(err)
	}
	return certificate, nil
}

// TrimError removes the middle of over-long error message strings
func TrimError(err error) error {
	const MAX_LEN = 100
	message := fmt.Sprintf("%s", err)
	if len(message) > MAX_LEN {
		return errors.New(message[:MAX_LEN/2] + "..." + message[len(message)-MAX_LEN/2:])
	}
	return err
}

// ContextError prefixes an error message with the current function name
func ContextError(err error) error {
	if err == nil {
		return nil
	}
	pc, _, line, _ := runtime.Caller(1)
	funcName := runtime.FuncForPC(pc).Name()
	index := strings.LastIndex(funcName, "/")
	if index != -1 {
		funcName = funcName[index+1:]
	}
	return fmt.Errorf("%s#%d: %s", funcName, line, err)
}

// IsNetworkBindError returns true when the err is due to EADDRINUSE.
func IsNetworkBindError(err error) bool {
	return strings.Contains(err.Error(), "bind: address already in use")
}

// NoticeConsoleRewriter consumes JOSN-format notice input and parses each
// notice and rewrites in a more human-readable format more suitable for
// console output. The data payload field is left as JSON.
type NoticeConsoleRewriter struct {
	mutex  sync.Mutex
	writer io.Writer
	buffer []byte
}

// NewNoticeConsoleRewriter initializes a new NoticeConsoleRewriter
func NewNoticeConsoleRewriter(writer io.Writer) *NoticeConsoleRewriter {
	return &NoticeConsoleRewriter{writer: writer}
}

// Write implements io.Writer.
func (rewriter *NoticeConsoleRewriter) Write(p []byte) (n int, err error) {
	rewriter.mutex.Lock()
	defer rewriter.mutex.Unlock()

	rewriter.buffer = append(rewriter.buffer, p...)

	index := bytes.Index(rewriter.buffer, []byte("\n"))
	if index == -1 {
		return len(p), nil
	}
	line := rewriter.buffer[:index]
	rewriter.buffer = rewriter.buffer[index+1:]

	type NoticeObject struct {
		NoticeType string          `json:"noticeType"`
		Data       json.RawMessage `json:"data"`
		Timestamp  string          `json:"timestamp"`
	}

	var noticeObject NoticeObject
	_ = json.Unmarshal(line, &noticeObject)
	fmt.Fprintf(os.Stderr,
		"%s %s %s\n",
		noticeObject.Timestamp,
		noticeObject.NoticeType,
		string(noticeObject.Data))

	return len(p), nil
}
