// RAINBOND, Application Management Platform
// Copyright (C) 2020-2020 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package util

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond-oam/pkg/util/zip"
	"github.com/google/uuid"
)

//NewUUID new uuid string
func NewUUID() string {
	return strings.Replace(uuid.New().String(), "-", "", -1)
}

var reg = regexp.MustCompile(`(?U)\$\{.*\}`)

//ParseVariable parse and replace variable in source str
func ParseVariable(source string, configs map[string]string) string {
	resultKey := reg.FindAllString(source, -1)
	for _, sourcekey := range resultKey {
		key, defaultValue := getVariableKey(sourcekey)
		if value, ok := configs[key]; ok {
			source = strings.Replace(source, sourcekey, value, -1)
		} else if defaultValue != "" {
			source = strings.Replace(source, sourcekey, defaultValue, -1)
		}
	}
	return source
}

func getVariableKey(source string) (key, value string) {
	if len(source) < 4 {
		return "", ""
	}
	left := strings.Index(source, "{")
	right := strings.Index(source, "}")
	k := source[left+1 : right]
	if strings.Contains(k, ":") {
		re := strings.Split(k, ":")
		if len(re) > 1 {
			return re[0], re[1]
		}
		return re[0], ""
	}
	return k, ""
}

//Unzip archive file to target dir
func Unzip(archive, target string) error {
	reader, err := zip.OpenDirectReader(archive)
	if err != nil {
		return fmt.Errorf("error opening archive: %v", err)
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}
	for _, file := range reader.File {
		run := func() error {
			path := filepath.Join(target, file.Name)
			if file.FileInfo().IsDir() {
				os.MkdirAll(path, file.Mode())
				if file.Comment != "" && strings.Contains(file.Comment, "/") {
					guid := strings.Split(file.Comment, "/")
					if len(guid) == 2 {
						uid, _ := strconv.Atoi(guid[0])
						gid, _ := strconv.Atoi(guid[1])
						if err := os.Chown(path, uid, gid); err != nil {
							return fmt.Errorf("error changing owner: %v", err)
						}
					}
				}
				return nil
			}

			fileReader, err := file.Open()
			if err != nil {
				return fmt.Errorf("fileReader; error opening file: %v", err)
			}
			defer fileReader.Close()
			targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
			if err != nil {
				return fmt.Errorf("targetFile; error opening file: %v", err)
			}
			defer targetFile.Close()

			if _, err := io.Copy(targetFile, fileReader); err != nil {
				return fmt.Errorf("error copy file: %v", err)
			}
			if file.Comment != "" && strings.Contains(file.Comment, "/") {
				guid := strings.Split(file.Comment, "/")
				if len(guid) == 2 {
					uid, _ := strconv.Atoi(guid[0])
					gid, _ := strconv.Atoi(guid[1])
					if err := os.Chown(path, uid, gid); err != nil {
						return err
					}
				}
			}
			return nil
		}
		if err := run(); err != nil {
			return err
		}
	}

	return nil
}

//Untar tar -zxvf
func Untar(archive, target string) error {
	cmd := exec.Command("tar", "-xzf", archive, "-C", target)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

//UnImagetar image-tar
func UnImagetar(archive, target string) error {
	cmd := exec.Command("tar", "-xf", archive, "-C", target)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

//GetFileList -
func GetFileList(dirpath string, level int) ([]string, error) {
	var dirlist []string
	list, err := ioutil.ReadDir(dirpath)
	if err != nil {
		return nil, err
	}
	for _, f := range list {
		if !f.IsDir() && level <= 1 {
			dirlist = append(dirlist, filepath.Join(dirpath, f.Name()))
		} else if level > 1 && f.IsDir() {
			list, err := GetFileList(filepath.Join(dirpath, f.Name()), level-1)
			if err != nil {
				return nil, err
			}
			dirlist = append(dirlist, list...)
		}
	}
	return dirlist, nil
}

// ReadJson -
func ReadJson(filePath string) (result string) {
	file, err := os.Open(filePath)
	defer file.Close()
	if err != nil {
		fmt.Println("ERROR:", err)
	}
	buf := bufio.NewReader(file)
	for {
		s, err := buf.ReadString('\n')
		result += s
		if err != nil {
			if err == io.EOF {
				fmt.Println("Read is ok")
				break
			} else {
				fmt.Println("ERROR:", err)
				return
			}
		}
	}
	return result
}

// FormatPath format path
func FormatPath(s string) string {
	log.Println("runtime.GOOS:", runtime.GOOS)
	switch runtime.GOOS {
	case "windows":
		return strings.Replace(s, "/", "\\", -1)
	case "darwin", "linux":
		return strings.Replace(s, "\\", "/", -1)
	default:
		fmt.Println("only support linux,windows,darwin, but os is " + runtime.GOOS)
		return s
	}
}

// CopyDir copy dir
func CopyDir(src string, dest string) error {
	_, err := os.Stat(dest)
	if err != nil {
		if !os.IsExist(err) {
			err := os.MkdirAll(dest, 0755)
			if err != nil {
				fmt.Println("make and copy dir error", err)
				return err
			}
		}
	}
	src = FormatPath(src)
	dest = FormatPath(dest)

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("xcopy", src, dest, "/I", "/E")
	case "darwin", "linux":
		cmd = exec.Command("cp", "-R", src, dest)
	}
	outPut, err := cmd.Output()
	if err != nil {
		fmt.Println("Output error: %s", err.Error())
		return err
	}
	fmt.Println(outPut)
	return nil
}
