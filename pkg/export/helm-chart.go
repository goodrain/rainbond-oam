package export

import (
	"fmt"
	"github.com/goodrain/rainbond-oam/pkg/ram/v1alpha1"
	"github.com/goodrain/rainbond-oam/pkg/util/image"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os"
	"path"
	"sigs.k8s.io/yaml"
	"time"
)

var (
	ImagePush string = "image_push"
	ImageSave string = "image_save"
)

type helmChartExporter struct {
	logger      *logrus.Logger
	ram         v1alpha1.RainbondApplicationConfig
	imageClient image.Client
	mode        string
	homePath    string
	exportPath  string
}

func (h *helmChartExporter) Export() (*Result, error) {
	h.logger.Infof("start export app %s to helm chart spec", h.ram.AppName)
	if err := SaveComponents(h.ram, h.imageClient, h.exportPath, h.logger); err != nil {
		h.logger.Errorf("helm chart export save component failure %v", err)
		return nil, err
	}
	h.logger.Infof("success save components")
	// Save plugin attachments
	if err := SavePlugins(h.ram, h.imageClient, h.exportPath, h.logger); err != nil {
		return nil, err
	}
	h.logger.Infof("success save plugins")
	if err := h.initHelmChart(); err != nil {
		return nil, err
	}
	packageName := fmt.Sprintf("%s-%s-helm.tar.gz", h.ram.AppName, h.ram.AppVersion)
	name, err := Packaging(packageName, h.homePath, h.exportPath)
	if err != nil {
		err = fmt.Errorf("Failed to package app %s: %s ", packageName, err.Error())
		h.logger.Error(err)
		return nil, err
	}
	h.logger.Infof("success export app " + h.ram.AppName)
	return &Result{PackagePath: path.Join(h.homePath, name), PackageName: name}, nil
}

func (h *helmChartExporter) initHelmChart() error {
	helmChartPath := path.Join(h.exportPath, h.ram.AppName)
	err := h.writeChartYaml(helmChartPath)
	if err != nil {
		h.logger.Errorf("%v writeChartYaml failure %v", h.ram.AppName, err)
		return err
	}
	h.logger.Infof("writeChartYaml success")
	for i := 0; i < 40; i++ {
		time.Sleep(1 * time.Second)
		if CheckFileExist(path.Join(helmChartPath, "end.yaml")) {
			h.logger.Infof("end.yaml creeate success")
			break
		}
	}

	return h.writeTemplateYaml(helmChartPath)
}

type ChartYaml struct {
	ApiVersion  string `json:"apiVersion,omitempty"`
	AppVersion  string `json:"appVersion,omitempty"`
	Description string `json:"description,omitempty"`
	Name        string `json:"name,omitempty"`
	Type        string `json:"type,omitempty"`
	Version     string `json:"version,omitempty"`
}

func (h *helmChartExporter) writeChartYaml(helmChartPath string) error {
	cy := ChartYaml{
		ApiVersion:  "v2",
		AppVersion:  h.ram.AppVersion,
		Description: h.ram.Annotations["version_info"],
		Name:        h.ram.AppName,
		Type:        "application",
		Version:     h.ram.AppVersion,
	}
	cyYaml, err := yaml.Marshal(cy)
	if err != nil {
		return err
	}
	return h.write(path.Join(helmChartPath, "Chart.yaml"), cyYaml)
}

func (h *helmChartExporter) writeTemplateYaml(helmChartPath string) error {
	helmChartTemplatePath := path.Join(helmChartPath, "templates")
	for _, k8sResource := range h.ram.K8sResources {
		var unstructuredObject unstructured.Unstructured
		err := yaml.Unmarshal([]byte(k8sResource.Content), &unstructuredObject)
		if err != nil {
			return err
		}
		unstructuredObject.SetNamespace("")
		unstructuredObject.SetResourceVersion("")
		unstructuredObject.SetCreationTimestamp(metav1.Time{})
		unstructuredObject.SetUID("")
		unstructuredYaml, err := yaml.Marshal(unstructuredObject)
		if err != nil {
			return err
		}
		err = h.write(path.Join(helmChartTemplatePath, fmt.Sprintf("%v.yaml", unstructuredObject.GetKind())), unstructuredYaml)
		if err != nil {
			return err
		}
	}
	return nil
}

func CheckFileExist(fileName string) bool {
	_, err := os.Stat(fileName)
	return !os.IsNotExist(err)
}

func (h *helmChartExporter) write(helmChartFilePath string, meta []byte) error {
	var fl *os.File
	var err error
	if CheckFileExist(helmChartFilePath) {
		fl, err = os.OpenFile(helmChartFilePath, os.O_APPEND|os.O_WRONLY, 0755)
		if err != nil {
			return err
		}
	} else {
		fl, err = os.Create(helmChartFilePath)
		if err != nil {
			return err
		}
	}
	defer fl.Close()
	n, err := fl.Write(append(meta, []byte("\n---\n")...))
	if err != nil {
		return err
	}
	if n < len(append(meta, []byte("\n---")...)) {
		return fmt.Errorf("write insufficient length")
	}
	return nil
}
