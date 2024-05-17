package cpk

type PackageJSONCPK struct {
	Architecture   string     `json:"architecture"`
	Browser        Browser    `json:"browser"`
	Category       string     `json:"category"`
	Classification string     `json:"classification"`
	Count          int        `json:"count"`
	Description    string     `json:"description"`
	Genericname    string     `json:"genericname"`
	Glibc          string     `json:"glibc"`
	Id             string     `json:"id"`
	Name           string     `json:"name"`
	News           string     `json:"news"`
	Os             string     `json:"os"`
	Permission     Permission `json:"permission"`
	Runtime        string     `json:"runtime"`
	Scripts        Scripts    `json:"scripts"`
	Search         string     `json:"search"`
	Secret         string     `json:"secret"`
	Size           string     `json:"size"`
	Start          string     `json:"start"`
	Summary        string     `json:"summary"`
	Todo           string     `json:"todo"`
	Type           string     `json:"type"`
	Vendor         Vendor     `json:"vendor"`
	Version        string     `json:"version"`
	Web            Web        `json:"web"`
}

type Browser struct {
	Height string `json:"height"`
	Width  string `json:"width"`
	X      string `json:"x"`
	Y      string `json:"y"`
}

type Permission struct {
	Dbus       bool   `json:"dbus"`
	Display    bool   `json:"display"`
	Filesystem string `json:"filesystem"`
	Ipc        bool   `json:"ipc"`
	Network    bool   `json:"network"`
	Root       bool   `json:"root"`
}

type Scripts struct {
	Enter    string `json:"enter"`
	Postinst string `json:"postinst"`
	Postrm   string `json:"postrm"`
	Postup   string `json:"postup"`
	Preinst  string `json:"preinst"`
	Prerm    string `json:"prerm"`
	Prestart string `json:"prestart"`
	Preup    string `json:"preup"`
}

type Vendor struct {
	Description string `json:"description"`
	Email       string `json:"email"`
	Homepage    string `json:"homepage"`
	Name        string `json:"name"`
	Telephone   string `json:"telephone"`
}

type Web struct {
	Application string     `json:"application"`
	Database    Database   `json:"database"`
	Middleware  Middleware `json:"middleware"`
	Runtime     Runtime    `json:"runtime"`
}

type Database struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Middleware struct {
	Extensions string `json:"extensions"`
	Name       string `json:"name"`
	Version    string `json:"version"`
}

type Runtime struct {
	Extensions string `json:"extensions"`
	Name       string `json:"name"`
	Version    string `json:"version"`
}
