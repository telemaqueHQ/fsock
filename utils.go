/*
utils.go is released under the MIT License <http://www.opensource.org/licenses/mit-license.php
Copyright (C) ITsysCOM. All Rights Reserved.

Provides FreeSWITCH socket communication.
*/
package fsock

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"
)

const EventBodyTag = "EvBody"

type logger interface {
	Alert(string) error
	Close() error
	Crit(string) error
	Debug(string) error
	Emerg(string) error
	Err(string) error
	Info(string) error
	Notice(string) error
	Warning(string) error
}
type nopLogger struct{}

func (nopLogger) Alert(string) error   { return nil }
func (nopLogger) Close() error         { return nil }
func (nopLogger) Crit(string) error    { return nil }
func (nopLogger) Debug(string) error   { return nil }
func (nopLogger) Emerg(string) error   { return nil }
func (nopLogger) Err(string) error     { return nil }
func (nopLogger) Info(string) error    { return nil }
func (nopLogger) Notice(string) error  { return nil }
func (nopLogger) Warning(string) error { return nil }

// Convert fseventStr into fseventMap
func FSEventStrToMap(fsevstr string, headers []string) map[string]string {
	fsevent := make(map[string]string)
	filtered := (len(headers) != 0)
	for _, strLn := range strings.Split(fsevstr, "\n") {
		if hdrVal := strings.SplitN(strLn, ": ", 2); len(hdrVal) == 2 {
			if filtered && isSliceMember(headers, hdrVal[0]) {
				continue // Loop again since we only work on filtered fields
			}
			fsevent[hdrVal[0]] = urlDecode(strings.TrimSpace(strings.TrimRight(hdrVal[1], "\n")))
		}
	}
	return fsevent
}

// Converts string received from fsock into a list of channel info, each represented in a map
func MapChanData(chanInfoStr string) (chansInfoMap []map[string]string) {
	chansInfoMap = make([]map[string]string, 0)
	spltChanInfo := strings.Split(chanInfoStr, "\n")
	if len(spltChanInfo) <= 4 {
		return
	}
	hdrs := strings.Split(spltChanInfo[0], ",")
	for _, chanInfoLn := range spltChanInfo[1 : len(spltChanInfo)-3] {
		chanInfo := splitIgnoreGroups(chanInfoLn, ",")
		if len(hdrs) != len(chanInfo) {
			continue
		}
		chnMp := make(map[string]string)
		for iHdr, hdr := range hdrs {
			chnMp[hdr] = chanInfo[iHdr]
		}
		chansInfoMap = append(chansInfoMap, chnMp)
	}
	return
}

func EventToMap(event string) (result map[string]string) {
	result = make(map[string]string)
	body := false
	spltevent := strings.Split(event, "\n")
	for i := 0; i < len(spltevent); i++ {
		if len(spltevent[i]) == 0 {
			body = true
			continue
		}
		if body {
			result[EventBodyTag] = strings.Join(spltevent[i:], "\n")
			return
		}
		if val := strings.SplitN(spltevent[i], ": ", 2); len(val) == 2 {
			result[val[0]] = urlDecode(strings.TrimSpace(val[1]))
		}
	}
	return
}

// helper function for uuid generation
func genUUID() string {
	b := make([]byte, 16)
	io.ReadFull(rand.Reader, b)
	b[6] = (b[6] & 0x0F) | 0x40
	b[8] = (b[8] &^ 0x40) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10],
		b[10:])
}

func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// splitIgnoreGroups splits input string by specified separator while ignoring elements grouped using "{}", "[]", or "()"
func splitIgnoreGroups(s string, sep string) (sl []string) {
	if s == "" {
		return []string{}
	}
	if sep == "" {
		return []string{s}
	}
	var idx, sqBrackets, crlBrackets, parantheses int
	for i, ch := range s {
		if s[i] == sep[0] && sqBrackets == 0 && crlBrackets == 0 && parantheses == 0 {
			sl = append(sl, s[idx:i])
			idx = i + 1
		} else if ch == '[' {
			sqBrackets++
		} else if ch == ']' && sqBrackets > 0 {
			sqBrackets--
		} else if ch == '{' {
			crlBrackets++
		} else if ch == '}' && crlBrackets > 0 {
			crlBrackets--
		} else if ch == '(' {
			parantheses++
		} else if ch == ')' && parantheses > 0 {
			parantheses--
		}
	}
	sl = append(sl, s[idx:])
	return
}

// Extracts value of a header from anywhere in content string
func headerVal(hdrs, hdr string) string {
	var hdrSIdx, hdrEIdx int
	if hdrSIdx = strings.Index(hdrs, hdr); hdrSIdx == -1 {
		return ""
	} else if hdrEIdx = strings.Index(hdrs[hdrSIdx:], "\n"); hdrEIdx == -1 {
		hdrEIdx = len(hdrs[hdrSIdx:])
	}
	splt := strings.SplitN(hdrs[hdrSIdx:hdrSIdx+hdrEIdx], ": ", 2)
	if len(splt) != 2 {
		return ""
	}
	return strings.TrimSpace(strings.TrimRight(splt[1], "\n"))
}

// FS event header values are urlencoded. Use this to decode them. On error, use original value
func urlDecode(hdrVal string) string {
	if valUnescaped, errUnescaping := url.QueryUnescape(hdrVal); errUnescaping == nil {
		hdrVal = valUnescaped
	}
	return hdrVal
}

func getMapKeys(m map[string][]func(string, int)) (keys []string) {
	keys = make([]string, len(m))
	indx := 0
	for key := range m {
		keys[indx] = key
		indx++
	}
	return
}

// Binary string search in slice
func isSliceMember(ss []string, s string) bool {
	sort.Strings(ss)
	i := sort.SearchStrings(ss, s)
	return (i < len(ss) && ss[i] == s)
}

// fibDuration returns successive Fibonacci numbers converted to time.Duration.
func fibDuration(durationUnit, maxDuration time.Duration) func() time.Duration {
	a, b := 0, 1
	return func() time.Duration {
		a, b = b, a+b
		fibNrAsDuration := time.Duration(a) * durationUnit
		if maxDuration > 0 && maxDuration < fibNrAsDuration {
			return maxDuration
		}
		return fibNrAsDuration
	}
}
