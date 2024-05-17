package cpk

type ImageJsonCPK struct {
	Apps []Apps `json:"apps"`
	Id   string `json:"id"`
}

type Apps struct {
	CMD          string            `json:"cmd"`
	Constraints  [][]string        `json:"constraints"`
	Container    Container         `json:"container"`
	Cpus         float64           `json:"cpus"`
	Dependencies []string          `json:"dependencies"`
	Disk         int               `json:"disk"`
	HealthChecks []HealthChecks    `json:"healthChecks"`
	ID           string            `json:"id"`
	Instances    int               `json:"instances"`
	Labels       map[string]string `json:"labels"`
	ENV          map[string]string `json:"env"`
	Mem          int               `json:"mem"`
}

type Container struct {
	Docker  Docker   `json:"docker"`
	Type    string   `json:"type"`
	Volumes []Volume `json:"volumes"`
}

type Docker struct {
	ForcePullImage bool          `json:"forcePullImage"`
	Image          string        `json:"image"`
	Network        string        `json:"network"`
	Parameters     []Parameter   `json:"parameters"`
	PortMappings   []PortMapping `json:"portMappings"`
	Privileged     bool          `json:"privileged"`
}

type Parameter struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type PortMapping struct {
	ContainerPort int               `json:"containerPort"`
	HostPort      int               `json:"hostPort"`
	Labels        map[string]string `json:"labels"`
	Name          string            `json:"name"`
	Protocol      string            `json:"protocol"`
	ServicePort   int               `json:"servicePort"`
}

type Volume struct {
	ContainerPath string `json:"containerPath"`
	HostPath      string `json:"hostPath"`
	Mode          string `json:"mode"`
}

type HealthChecks struct {
	GracePeriodSeconds     int    `json:"gracePeriodSeconds"`
	IgnoreHttp1Xx          bool   `json:"ignoreHttp1xx"`
	IntervalSeconds        int    `json:"intervalSeconds"`
	MaxConsecutiveFailures int    `json:"maxConsecutiveFailures"`
	Path                   string `json:"path"`
	PortIndex              int    `json:"portIndex"`
	Protocol               string `json:"protocol"`
	TimeoutSeconds         int    `json:"timeoutSeconds"`
}
