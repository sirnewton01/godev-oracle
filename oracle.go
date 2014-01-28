// Copyright 2013 Chris McGee <sirnewton_01@yahoo.ca>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/build"
	"log"
	"net/http"
	"net/http/cgi"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var (
	goroot  = ""
	srcDirs = []string{}
	godev   = flag.Bool("godev", false, "Set the command into GoDev CGI mode")
)

func init() {
	flag.Parse()

	goroot = runtime.GOROOT() + string(os.PathSeparator)

	dirs := build.Default.SrcDirs()

	for i := len(dirs) - 1; i >= 0; i-- {
		srcDir := dirs[i]

		if !strings.HasPrefix(srcDir, goroot) {
			srcDirs = append(srcDirs, srcDir)
		}
	}
}

const (
	SEV_ERR  = "Error"
	SEV_WARN = "Warning"
	SEV_INFO = "Info"
	SEV_CNCL = "Cancel"
	SEV_OK   = "Ok"
)

// Orion Status object
type Status struct {
	// One of SEV_ERR, SEV_WARN, SEV_INFO, SEV_CNCL, SEV_OK
	Severity        string
	HttpCode        uint
	Message         string
	DetailedMessage string
}

// Helper function to write an Orion-compatible error message with an optional error object
func ShowError(writer http.ResponseWriter, httpCode uint, message string, err error) {
	writer.Header().Add("Content-Type", "application/json")
	writer.WriteHeader(int(httpCode))
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	status := Status{SEV_ERR, httpCode, message, errStr}
	bytes, err := json.Marshal(status)
	if err != nil {
		panic(err)
	}
	_, err = writer.Write(bytes)
	if err != nil {
		log.Printf("ERROR: %v\n", err)
	}
}

// Helper function to write an Orion-compatible JSON object
func ShowJson(writer http.ResponseWriter, httpCode uint, obj interface{}) {
	writer.Header().Add("Content-Type", "application/json")
	writer.WriteHeader(int(httpCode))

	bytes, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	_, err = writer.Write(bytes)
	if err != nil {
		log.Printf("ERROR %v\n", err)
	}
}

type ImplementsResult struct {
	Implements Implements `json:"implements"`
}

type Implements struct {
	FromPtr []ImplementsType `json:"fromptr,omitempty"`
	To      []ImplementsType `json:"to,omitempty"`
}

type ImplementsType struct {
	Name       string `json:"name"`
	Pos        string `json:"pos"`
	LogicalPos string `json:"logicalPos"`
	Kind       string `json:"kind"`
}

type ReferrersResult struct {
	Referrers Referrers `json:"referrers"`
}

type Referrers struct {
	LogicalRefs []string `json:"logicalRefs"`
	Refs        []string `json:"refs"`
}

type CallersResult struct {
	Callers []Caller `json:"callers"`
}

type Caller struct {
	Desc       string `json:"desc"`
	Pos        string `json:"pos"`
	LogicalPos string `json:"logicalPos"`
}

type PeersResult struct {
	Peers Peers `json:"peers"`
}

type Peers struct {
	LogicalAllocs   []string `json:"logicalAllocs"`
	Allocs          []string `json:"allocs"`
	LogicalReceives []string `json:"logicalReceives"`
	Receives        []string `json:"receives"`
	LogicalSends    []string `json:"logicalSends"`
	Sends           []string `json:"sends"`
}

func getLogicalPos(localPos string) (logicalPos string) {
	for _, path := range append(srcDirs, filepath.Join(goroot, "/src/pkg")) {
		match := path
		if match[len(match)-1] != filepath.Separator {
			match = match + string(filepath.Separator)
		}

		if strings.HasPrefix(localPos, match) {
			logicalPos = localPos[len(match)-1:]

			if path == filepath.Join(goroot, "/src/pkg") {
				logicalPos = "/GOROOT" + logicalPos
			}

			// Replace any Windows back-slashes into forward slashes
			logicalPos = strings.Replace(logicalPos, "\\", "/", -1)
		}
	}

	if logicalPos == "" {
		logicalPos = localPos
	}

	return logicalPos
}

func main() {
	if *godev == false {
		fmt.Printf("This is a CGI program mean to be plugged into the godev IDE\n")
		return
	}

	req, err := cgi.Request()

	if err != nil {
		fmt.Printf("Error trying to set up the CGI request", err.Error())
		return
	}

	writer := &cgiWriter{file: os.Stdout, wroteHeader: false, header: make(http.Header)}

	if !oracleHandler(writer, req, req.URL.Path, strings.Split(req.URL.Path, "/")[2:]) {
		ShowError(writer, 400, "Request not handled", nil)
	}
}

type cgiWriter struct {
	file        *os.File
	header      http.Header
	wroteHeader bool
}

func (writer *cgiWriter) WriteHeader(status int) {
	if !writer.wroteHeader {
		statusStr := strconv.FormatInt(int64(status), 10)
		writer.file.Write([]byte("Status: " + statusStr + "\r\n"))
		writer.header.Write(writer.file)

		// Final blank line separates the headers from the content
		writer.file.Write([]byte("\r\n"))
	} else {
		fmt.Errorf("Multiple write header calls")
	}
	writer.wroteHeader = true
}

func (writer *cgiWriter) Header() http.Header {
	return writer.header
}

func (writer *cgiWriter) Write(bytes []byte) (n int, err error) {
	if !writer.wroteHeader {
		writer.WriteHeader(http.StatusOK)
	}

	return writer.file.Write(bytes)
}

func oracleHandler(writer http.ResponseWriter, req *http.Request, path string, pathSegs []string) bool {
	switch {
	case req.Method == "GET" && pathSegs[2] == "implements":
		localFilePath := ""
		pathToMatch := "/" + strings.Join(pathSegs[4:], "/")
		pathToMatch = strings.Replace(pathToMatch, "/GOROOT/", "/", 1)

		for _, srcDir := range append(srcDirs, filepath.Join(goroot, "/src/pkg")) {
			path := filepath.Join(srcDir, pathToMatch)

			_, err := os.Stat(path)

			if err == nil {
				localFilePath = path
				break
			}
		}

		pos := req.URL.Query().Get("pos")
		pos = localFilePath + ":#" + pos

		scope := req.URL.Query().Get("scope")

		cmd := exec.Command("oracle", "-format=json", "-pos="+pos, "implements", scope)
		output, err := cmd.Output()

		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				// Executable was not found, inform the user
				ShowError(writer, 500, "Oracle tool failed or is not installed.", err)
				return true
			}

			// Tool returns no results basically
			writer.WriteHeader(204)
			return true
		}

		implements := &ImplementsResult{}
		json.Unmarshal(output, implements)

		for idx, _ := range implements.Implements.FromPtr {
			path := getLogicalPos(implements.Implements.FromPtr[idx].Pos)
			implements.Implements.FromPtr[idx].LogicalPos = path
		}

		for idx, _ := range implements.Implements.To {
			path := getLogicalPos(implements.Implements.To[idx].Pos)
			implements.Implements.To[idx].LogicalPos = path
		}

		ShowJson(writer, 200, implements)
		return true
	case req.Method == "GET" && pathSegs[2] == "referrers":
		localFilePath := ""
		pathToMatch := "/" + strings.Join(pathSegs[4:], "/")
		pathToMatch = strings.Replace(pathToMatch, "/GOROOT/", "/", 1)

		for _, srcDir := range append(srcDirs, filepath.Join(goroot, "/src/pkg")) {
			path := filepath.Join(srcDir, pathToMatch)

			_, err := os.Stat(path)

			if err == nil {
				localFilePath = path
				break
			}
		}

		pos := req.URL.Query().Get("pos")
		pos = localFilePath + ":#" + pos

		scope := req.URL.Query().Get("scope")

		cmd := exec.Command("oracle", "-format=json", "-pos="+pos, "referrers", scope)
		output, err := cmd.StdoutPipe()
		if err != nil {
			ShowError(writer, 500, "Error opening pipe", err)
			return true
		}
		defer output.Close()
		err = cmd.Start()

		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				// Executable was not found, inform the user
				ShowError(writer, 500, "Oracle tool failed or is not installed.", err)
				return true
			}

			// Tool returns no results basically
			writer.WriteHeader(204)
			return true
		}

		decoder := json.NewDecoder(output)
		referrers := &ReferrersResult{}
		decoder.Decode(referrers)

		cmd.Wait()

		for idx, _ := range referrers.Referrers.Refs {
			path := getLogicalPos(referrers.Referrers.Refs[idx])
			referrers.Referrers.LogicalRefs = append(referrers.Referrers.LogicalRefs, path)
		}

		ShowJson(writer, 200, referrers)
		return true
	case req.Method == "GET" && pathSegs[2] == "callers":
		localFilePath := ""
		pathToMatch := "/" + strings.Join(pathSegs[4:], "/")
		pathToMatch = strings.Replace(pathToMatch, "/GOROOT/", "/", 1)

		for _, srcDir := range append(srcDirs, filepath.Join(goroot, "/src/pkg")) {
			path := filepath.Join(srcDir, pathToMatch)

			_, err := os.Stat(path)

			if err == nil {
				localFilePath = path
				break
			}
		}

		pos := req.URL.Query().Get("pos")
		pos = localFilePath + ":#" + pos

		scope := req.URL.Query().Get("scope")

		cmd := exec.Command("oracle", "-format=json", "-pos="+pos, "callers", scope)
		output, err := cmd.StdoutPipe()
		if err != nil {
			ShowError(writer, 500, "Error opening pipe", err)
			return true
		}
		defer output.Close()
		err = cmd.Start()

		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				// Executable was not found, inform the user
				ShowError(writer, 500, "Oracle tool failed or is not installed.", err)
				return true
			}

			// Tool returns no results basically
			writer.WriteHeader(204)
			return true
		}

		decoder := json.NewDecoder(output)
		callers := &CallersResult{}
		decoder.Decode(callers)

		cmd.Wait()

		for idx, _ := range callers.Callers {
			callers.Callers[idx].LogicalPos = getLogicalPos(callers.Callers[idx].Pos)
		}

		ShowJson(writer, 200, callers)
		return true
	case req.Method == "GET" && pathSegs[2] == "peers":
		localFilePath := ""
		pathToMatch := "/" + strings.Join(pathSegs[4:], "/")
		pathToMatch = strings.Replace(pathToMatch, "/GOROOT/", "/", 1)

		for _, srcDir := range append(srcDirs, filepath.Join(goroot, "/src/pkg")) {
			path := filepath.Join(srcDir, pathToMatch)

			_, err := os.Stat(path)

			if err == nil {
				localFilePath = path
				break
			}
		}

		pos := req.URL.Query().Get("pos")
		pos = localFilePath + ":#" + pos

		scope := req.URL.Query().Get("scope")

		cmd := exec.Command("oracle", "-format=json", "-pos="+pos, "peers", scope)
		output, err := cmd.StdoutPipe()
		if err != nil {
			ShowError(writer, 500, "Error opening pipe", err)
			return true
		}
		defer output.Close()
		err = cmd.Start()

		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				// Executable was not found, inform the user
				ShowError(writer, 500, "Oracle tool failed or is not installed.", err)
				return true
			}

			// Tool returns no results basically
			writer.WriteHeader(204)
			return true
		}

		decoder := json.NewDecoder(output)
		peers := &PeersResult{}
		decoder.Decode(peers)

		cmd.Wait()

		for idx, _ := range peers.Peers.Allocs {
			path := getLogicalPos(peers.Peers.Allocs[idx])
			peers.Peers.LogicalAllocs = append(peers.Peers.LogicalAllocs, path)
		}

		for idx, _ := range peers.Peers.Sends {
			path := getLogicalPos(peers.Peers.Sends[idx])
			peers.Peers.LogicalSends = append(peers.Peers.LogicalSends, path)
		}

		for idx, _ := range peers.Peers.Receives {
			path := getLogicalPos(peers.Peers.Receives[idx])
			peers.Peers.LogicalReceives = append(peers.Peers.LogicalReceives, path)
		}

		ShowJson(writer, 200, peers)
		return true
	}

	return false
}
