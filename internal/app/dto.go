package app

import "time"

type VolumeDTO struct {
	Root          string     `json:"root"`
	Label         string     `json:"label"`
	FS            string     `json:"fs"`
	Type          string     `json:"type"`
	Total         int64      `json:"total"`
	Free          int64      `json:"free"`
	HasRecycleBin bool       `json:"hasRecycleBin"`
	LastScan      *time.Time `json:"lastScan"`
	EstimatedJunk int64      `json:"estimatedJunk"`
}

type ProgressDTO struct {
	Volume      string  `json:"volume"`
	Files       int64   `json:"files"`
	Dirs        int64   `json:"dirs"`
	Bytes       int64   `json:"bytes"`
	Denied      int64   `json:"denied"`
	Current     string  `json:"current"`
	BytesPerSec float64 `json:"bytesPerSec"`
}

type ScanDoneDTO struct {
	Volume       string   `json:"volume"`
	DurationMs   int64    `json:"durationMs"`
	Error        string   `json:"error"`
	DeniedCount  int      `json:"deniedCount"`
	DeniedSample []string `json:"deniedSample"`
}

type AIOpinionDTO struct {
	Verdict    string  `json:"verdict"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
}

type NodeDTO struct {
	ID          int32         `json:"id"`
	Name        string        `json:"name"`
	Path        string        `json:"path"`
	Size        int64         `json:"size"`
	Files       uint32        `json:"files"`
	LatestMod   int64         `json:"latestMod"`
	HasChildren bool          `json:"hasChildren"`
	Link        bool          `json:"link"`
	Cloud       bool          `json:"cloud"`
	Denied      bool          `json:"denied"`
	Inactive    bool          `json:"inactive"`
	Verdict     string        `json:"verdict"`
	Category    string        `json:"category"`
	Source      string        `json:"source"`
	RuleID      string        `json:"ruleId"`
	Action      string        `json:"action"`
	Explain     string        `json:"explain"`
	Regenerable string        `json:"regenerable"`
	Command     string        `json:"command"`
	AI          *AIOpinionDTO `json:"ai"`
}

type FileDTO struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
	Verdict string `json:"verdict"`
	RuleID  string `json:"ruleId"`
}

type ExtDTO struct {
	Ext   string `json:"ext"`
	Count uint64 `json:"count"`
	Bytes int64  `json:"bytes"`
}

type CategoryDTO struct {
	Category string `json:"category"`
	Bytes    int64  `json:"bytes"`
	Count    int    `json:"count"`
}

type SummaryDTO struct {
	Volume       string        `json:"volume"`
	SafeBytes    int64         `json:"safeBytes"`
	CautionBytes int64         `json:"cautionBytes"`
	KeepBytes    int64         `json:"keepBytes"`
	ByCategory   []CategoryDTO `json:"byCategory"`
}
