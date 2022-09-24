// RAINBOND, Application Management Platform
// Copyright (C) 2020-2020 Goodrain Co., Ltr.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltr.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package export

import (
	"encoding/json"
	"fmt"
	"github.com/goodrain/rainbond-oam/pkg/util"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/goodrain/rainbond-oam/pkg/ram/v1alpha1"
	"github.com/sirupsen/logrus"
)

const sourceCode = "source_code"

type slugExporter struct {
	logger     *logrus.Logger
	ram        v1alpha1.RainbondApplicationConfig
	ctr        ContainerdAPI
	mode       string
	homePath   string
	exportPath string
}

func (s *slugExporter) Export() (*Result, error) {
	s.logger.Infof("start export app %s to ram app spec", s.ram.AppName)
	// Delete the old application group directory and then regenerate the application package
	if err := PrepareExportDir(s.exportPath); err != nil {
		s.logger.Errorf("prepare export dir failure %s", err.Error())
		return nil, err
	}
	s.logger.Infof("success prepare export dir")
	if s.mode == "offline" {
		// Save components attachments
		if err := s.saveComponents(); err != nil {
			return nil, err
		}
		s.logger.Infof("success save components")
	}
	// UnTar component-images
	ciTarPath := fmt.Sprintf("%s/component-images.tar", s.exportPath)
	ciFilePath := fmt.Sprintf("%s/component-images", s.exportPath)
	err := os.Mkdir(ciFilePath, 0755)
	if err != nil {
		s.logger.Error("mkdir component-image error", err)
	}
	err = util.UnImagetar(ciTarPath, ciFilePath)
	if err != nil {
		s.logger.Error("component-images UnTar error", err)
		return nil, err
	}
	// get slug and env file and run script
	for _, component := range s.ram.Components {
		if component.ServiceSource == sourceCode {
			// Unmarshal manifest.json
			mfJsonPath := fmt.Sprintf("%s/manifest.json", ciFilePath)
			mfByte := util.ReadJson(mfJsonPath)
			var mfs []*v1alpha1.Manifest
			err = json.Unmarshal([]byte(mfByte), &mfs)
			if err != nil {
				s.logger.Error("mfs json Unmarshal error", err)
				return nil, err
			}
			for _, mf := range mfs {
				for _, tag := range mf.RepoTags {
					if tag == component.ShareImage {
						// Gets the Layer directory where slug is stored
						layer := mf.Layers[len(mf.Layers)-1]
						layerID := strings.Split(layer, "/")[0]
						layerPath := fmt.Sprintf("%s/%s", ciFilePath, layerID)
						layerTar := fmt.Sprintf("%s/%s", ciFilePath, layer)
						// UnTar layer
						err = util.UnImagetar(layerTar, layerPath)
						if err != nil {
							s.logger.Error("layer UnTar error", err)
							return nil, err
						}
						// Create a package path to store slug
						slugPath := fmt.Sprintf("%s/%s", s.exportPath, component.ServiceCname)
						err = os.Mkdir(slugPath, 0755)
						if err != nil {
							s.logger.Error("mkdir slug error", err)
							return nil, err
						}
						// Copy slug to store path
						slugOldPath := fmt.Sprintf("%s/tmp/slug/slug.tgz", layerPath)
						err = util.CopyDir(slugOldPath, slugPath)
						if err != nil {
							s.logger.Error("copy slug error", err)
							return nil, err
						}
						slugName := fmt.Sprintf("%s-slug.tgz", component.ServiceCname)
						err = os.Rename(slugPath+"/slug.tgz", fmt.Sprintf("%s/%s", slugPath, slugName))
						if err != nil {
							logrus.Error("slug.tgz rename error")
						}
						// Add an environment variable file
						if err := s.writeEnvFile(component, slugPath, s.ram.AppConfigGroups); err != nil {
							return nil, err
						}
						// Add a script to run slug
						if err := s.writeRunScript(slugPath, component.ServiceCname); err != nil {
							return nil, err
						}
					}
				}
			}
		}
	}
	// remove component images file
	if err = os.RemoveAll(ciTarPath); err != nil {
		return nil, err
	}
	if err = os.RemoveAll(ciFilePath); err != nil {
		return nil, err
	}
	// Add a script to app
	if err := s.writeAppScript(s.exportPath, s.ram.AppName); err != nil {
		return nil, err
	}
	// packaging
	name, err := s.packaging()
	if err != nil {
		return nil, err
	}
	s.logger.Infof("success export app " + s.ram.AppName)
	return &Result{PackagePath: path.Join(s.homePath, name), PackageName: name}, nil
}

func (s *slugExporter) saveComponents() error {
	var componentImageNames []string
	for _, component := range s.ram.Components {
		componentName := unicode2zh(component.ServiceCname)
		if component.ShareImage != "" {
			// app is image type
			localImageName, err := pullImage(s.ctr, component, s.logger)
			if err != nil {
				return err
			}
			s.logger.Infof("pull component %s image success", componentName)
			componentImageNames = append(componentImageNames, localImageName)
		}
	}
	start := time.Now()
	//ctx := context.Background()
	//err := docker.MultiImageSave(ctx, s.client, fmt.Sprintf("%s/component-images.tar", s.exportPath), componentImageNames...)
	w, err := os.Create(fmt.Sprintf("%s/component-images.tar", s.exportPath))
	if err != nil {
		logrus.Errorf("Failed to create file(%v) : %s", componentImageNames, err)
		return err
	}
	defer w.Close()
	err = saveImage(s.ctr, w, componentImageNames)
	if err != nil {
		logrus.Errorf("Failed to save image(%v) : %s", componentImageNames, err)
		return err
	}
	s.logger.Infof("save component images success, Take %s time", time.Now().Sub(start))
	return nil
}

func (s *slugExporter) writeEnvFile(component *v1alpha1.Component, slugPath string, AppConfigGroups []*v1alpha1.AppConfigGroup) error {
	// remove component  image hub info
	if s.mode == "offline" {
		for i := range s.ram.Components {
			s.ram.Components[i].AppImage = v1alpha1.ImageInfo{}
		}
	}
	var (
		fileKV    string
		envs      string
		configs   string
		connInfos string
	)
	// get env
	for _, env := range component.Envs {
		e := fmt.Sprintf("export %s=%s\n", env.AttrName, env.AttrValue)
		envs += e
	}
	// get config groups
	for _, AppConfigGroup := range AppConfigGroups {
		for _, key := range AppConfigGroup.ComponentKeys {
			if component.ComponentKey == key {
				for k, v := range AppConfigGroup.ConfigItems {
					config := fmt.Sprintf("export %s=%s\n", k, v)
					configs += config
				}
			}
		}
	}
	// get connection information
	for _, connectInfoMap := range component.ServiceConnectInfoMapList {
		connInfo := fmt.Sprintf("export %s=%s\n", connectInfoMap.AttrName, connectInfoMap.AttrValue)
		connInfos += connInfo
	}
	// get component port
	port := fmt.Sprintf("export %s=%v\n", "PORT", component.Ports[0].ContainerPort)

	// parameters are written to the file in KV format
	fileKV = envs + configs + connInfos + port

	txtName := fmt.Sprintf("%s.env", component.ServiceCname)
	envFile := path.Join(slugPath, txtName)
	f, err := os.OpenFile(envFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		s.logger.Error("open envFile error", err)
		return err
	}
	defer f.Close()
	err = ioutil.WriteFile(envFile, []byte(fileKV), 123)
	if err != nil {
		s.logger.Error("Write env to file error", err)
		return err
	}
	return nil
}

func (s *slugExporter) packaging() (string, error) {
	packageName := fmt.Sprintf("%s-%s-slug.tar.gz", s.ram.AppName, s.ram.AppVersion)

	cmd := exec.Command("tar", "-czf", path.Join(s.homePath, packageName), path.Base(s.exportPath))
	cmd.Dir = s.homePath
	if err := cmd.Run(); err != nil {
		err = fmt.Errorf("Failed to package app %s: %s ", packageName, err.Error())
		s.logger.Error(err)
		return "", err
	}
	return packageName, nil
}

func (s *slugExporter) writeRunScript(slugPath string, name string) error {
	shName := name + ".sh"
	shPath := path.Join(slugPath, shName)
	shfile, err := os.OpenFile(shPath, os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {

		logrus.Error("open script error")
		return err
	}
	defer shfile.Close()
	runScript := "#!/bin/bash\n\n###\n### app.sh — Controls app startup and stop.\n###\n### Usage:\n###   app.sh <Options>\n###\n### Options:\n###   start   Start your app.\n###   stop    Stop your app.\n###   status  Show app status.\n###   -h      Show this message.\n\n[ $DEBUG ] && set -x\n\n# make stdout colorful\nGREEN='\\033[1;32m'\nYELLOW='\\033[1;33m'\nRED='\\033[1;31m'\nNC='\\033[0m' # No Color\n\n# 定义当前服务组件的名字\nAPPNAME=$(basename $(pwd))\n\n# 定义当前工作目录\nHOME=$(pwd)\n\n\n# 解压 slug 包\nfunction processSlug() {\n    if [ -f ${APPNAME}-slug.tgz ]; then\n        tar xzf ${APPNAME}-slug.tgz -C $HOME\n    else\n        echo -e \"There is no slug file, ${0#*/} need it to start your app ...$RED Failure $NC\"\n        exit 1\n    fi\n}\n\n# 运行 .profile.d 中的所有文件\n# 这个过程会修改 PATH 环境变量\nfunction processRuntimeEnv() {\n    sleep 1\n    if [ -d .profile.d ]; then\n        echo -e \"Handling runtime environment ... $GREEN Done $NC\"\n        for file in .profile.d/*; do\n            source $file\n        done\n        hash -r\n    fi\n}\n\n# 导入用户自定义的其他环境变量\nfunction processCustomEnv() {\n    if [ -f ${APPNAME}.env ]; then\n        sleep 1\n        echo -e \"Handling custom environment ... $GREEN Done $NC\"\n        source ${APPNAME}.env\n    fi\n}\n\n# 处理启动命令\nfunction processCmd() {\n    # 从 Procfile 文件中截取\n    if [ -f Procfile ]; then\n        # 渲染启动命令中的环境变量\n        eval \"cat <<EOF\n$(<Procfile)\nEOF\n\" >${APPNAME}.cmd\n        sed -i 's/web: //' ${APPNAME}.cmd\n    elif [ ! -f Procfile ] && [ -s .release ]; then\n        eval \"cat <<EOF\n$(cat .release | grep web | sed 's/web: //')\nEOF\n\" >${APPNAME}.cmd\n    else\n        echo -e \"Can not detect start cmd, please check whether file Procfile or .release exists ... $RED Failure $NC\"\n        exit 1\n    fi\n}\n\n# 启动函数\nfunction appStart() {\n    appStatus >/dev/null 2>&1 &&\n        echo -e \"App ${APPNAME} is already running with pid $(cat ${APPNAME}.pid). Try exec $0 status\" &&\n        exit 1\n    processSlug\n    processRuntimeEnv\n    processCustomEnv\n    processCmd\n    echo \"Running app ${APPNAME}, you can check the logs in file ${APPNAME}.log\"\n    echo \"We will start your app with ==> $(cat ${APPNAME}.cmd)\"\n    nohup $(cat ${APPNAME}.cmd) >${APPNAME}.log 2>&1 &\n    # 对于进程运行过程中报错退出的，需要时间窗口来延迟检测\n    sleep 3\n    # 查询进程，来确定是否启动成功\n    RES=$(ps -p $! -o pid= -o comm=)\n    if [ ! -z \"$RES\" ]; then\n        echo -e \"Running app ${APPNAME} with process: $RES ... $GREEN Done $NC\"\n        echo $! >${APPNAME}.pid\n    else\n        echo -e \"Running app ${APPNAME} failed,check ${APPNAME}.log ... $RED Failure $NC\"\n    fi\n}\n\nfunction appStop() {\n    if [ -f ${APPNAME}.pid ]; then\n        PID=$(cat ${APPNAME}.pid)\n        if [ ! -z $PID ]; then\n            # For stopping Nginx process,SIGTERM is better than SIGKILL\n            kill -15 $PID >/dev/null 2>&1\n            if [ $? == 0 ]; then\n                echo -e \"Stopping app ${APPNAME} which running with pid ${PID} ... $GREEN Done $NC\"\n                rm -rf ${APPNAME}.pid\n            else\n                rm -rf ${APPNAME}.pid\n            fi\n        fi\n    else\n        echo \"The app ${APPNAME} is not running.Ignore the operation.\"\n    fi\n}\n\n# # TODO\n# function appRestart() {\n\n# }\n\n# 获取当前目录下的 app 是否启动\nfunction appStatus() {\n    PID=$(cat ${APPNAME}.pid 2>/dev/null)\n    RES=$(ps -p $PID -o pid= -o comm= 2>/dev/null)\n    if [ ! -z \"$RES\" ]; then\n        printf \"%-30s %-30s %-10s\\n\" AppName Status PID\n        printf \"%-30s \\e[1;32m%-30s\\e[m %-30s\\n\" ${APPNAME} \"Active(Running)\" $PID\n        return 0\n    else\n        printf \"%-30s %-30s %-30s\\n\" AppName Status PID\n        printf \"%-30s \\e[1;31m%-30s\\e[m %-30s\\n\" \"${APPNAME}\" \"Inactive(Exited)\" \"N/A\"\n        return 1\n    fi\n}\n\nfunction showHelp() {\n    sed -rn -e \"s/^### ?//p\" $0 | sed \"s#app.sh#${0}#g\"\n}\n\ncase $1 in\nstart)\n    appStart\n    ;;\nstop)\n    appStop\n    ;;\nstatus)\n    appStatus\n    ;;\n*)\n    showHelp\n    exit 1\n    ;;\nesac"
	err = ioutil.WriteFile(shPath, []byte(runScript), 0777)
	if err != nil {
		logrus.Error("write run script to sh error")
	}
	return nil
}

func (s *slugExporter) writeAppScript(appPath string, name string) error {
	shName := name + ".sh"
	shPath := path.Join(appPath, shName)
	shfile, err := os.OpenFile(shPath, os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {

		logrus.Error("open script error")
		return err
	}
	defer shfile.Close()
	appScript := "#!/bin/bash\n###\n### app.sh — Controls app startup and stop.\n###\n### Usage:\n###   app.sh <Options>\n###\n### Options:\n###   start   Start your app.\n###   stop    Stop your app.\n###   status  Show app status.\n###   -h      Show this message.\n\n[ $DEBUG ] && set -x\n\n# make stdout colorful\nGREEN='\\033[1;32m'\nYELLOW='\\033[1;33m'\nRED='\\033[1;31m'\nNC='\\033[0m' # No Color\n\n# 定义当前应用的名字\nAPPNAME=$(basename $(pwd))\n\n# 扫描当前应用中所有的服务组件名称\nAPPS=$(ls -d */ | sed \"s#\\/##g\")\n\n# 启动所有的服务组件\nfunction allAppStart() {\n    for app in ${APPS}; do\n        pushd $app >/dev/null 2>&1\n        ./$app.sh start | sed -n '$p'\n        popd >/dev/null 2>&1\n    done\n}\n\nfunction allAppStop() {\n    for app in ${APPS}; do\n        pushd $app >/dev/null 2>&1\n        ./$app.sh stop\n        popd >/dev/null 2>&1\n    done\n}\n\nfunction allAppStatus() {\n    printf \"%-30s %-30s %-10s\\n\" AppName Status PID\n    for app in ${APPS}; do\n        pushd $app >/dev/null 2>&1\n        ./$app.sh status | sed '1d'\n        popd >/dev/null 2>&1\n    done\n}\n\nfunction showHelp() {\n    sed -rn -e \"s/^### ?//p\" $0 | sed \"s#app.sh#${0}#g\"\n}\n\ncase $1 in\nstart)\n    allAppStart\n    ;;\nstop)\n    allAppStop\n    ;;\nstatus)\n    allAppStatus\n    ;;\n*)\n    showHelp\n    exit 1\n    ;;\nesac"
	err = ioutil.WriteFile(shPath, []byte(appScript), 0777)
	if err != nil {
		logrus.Error("write app script to sh error")
	}
	return nil
}
