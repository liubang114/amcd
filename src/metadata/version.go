package metadata

// Version 版本清单中的一条 / one entry of the version manifest
type Version struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	SHA1        string `json:"sha1"`
	ReleaseTime string `json:"releaseTime"`
}

// Manifest 版本清单 / version_manifest.json
type Manifest struct {
	Latest struct {
		Release  string `json:"release"`
		Snapshot string `json:"snapshot"`
	} `json:"latest"`
	Versions []Version `json:"versions"`
}

// Find 按版本 ID 查找 / look up a version by id
func (m *Manifest) Find(id string) (Version, bool) {
	for _, v := range m.Versions {
		if v.ID == id {
			return v, true
		}
	}
	return Version{}, false
}

// IsRelease 是否正式版 / whether the entry is a release
func (v Version) IsRelease() bool { return v.Type == "release" }

// Artifact 一个可下载对象 / a downloadable artifact
type Artifact struct {
	Path string `json:"path"`
	URL  string `json:"url"`
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
}

// AssetIndex 资源索引 / asset index entry
type AssetIndex struct {
	ID        string `json:"id"`
	SHA1      string `json:"sha1"`
	Size      int64  `json:"size"`
	TotalSize int64  `json:"totalSize"`
	URL       string `json:"url"`
}

// Library 依赖库 / a library entry
type Library struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Downloads struct {
		Artifact    Artifact            `json:"artifact"`
		Classifiers map[string]Artifact `json:"classifiers"`
	} `json:"downloads"`
	Rules   []Rule            `json:"rules"`
	Natives map[string]string `json:"natives"`
}

// Rule 规则条目 / a rules entry
type Rule struct {
	Action string `json:"action"`
	OS     struct {
		Name    string `json:"name"`
		Arch    string `json:"arch"`
		Version string `json:"version"`
	} `json:"os"`
}

// VersionJSON 完整版本 JSON / the full version json
type VersionJSON struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	MainClass    string `json:"mainClass"`
	Assets       string `json:"assets"`
	InheritsFrom string `json:"inheritsFrom"`
	JavaVersion  struct {
		Component    string `json:"component"`
		MajorVersion int    `json:"majorVersion"`
	} `json:"javaVersion"`
	Downloads struct {
		Client Artifact `json:"client"`
		Server Artifact `json:"server"`
	} `json:"downloads"`
	AssetIndex AssetIndex `json:"assetIndex"`
	Libraries  []Library  `json:"libraries"`
}
