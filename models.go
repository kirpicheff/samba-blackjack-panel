package main

type ADStatus struct {
	Joined bool   `json:"joined"`
	Info   string `json:"info"`
}

type ADJoinRequest struct {
	Realm    string `json:"realm"`
	Admin    string `json:"admin"`
	Password string `json:"password"`
}

type SambaStatus struct {
	Timestamp string              `json:"timestamp"`
	Version   string              `json:"version"`
	Sessions  map[string]Session  `json:"sessions"`
	Tcons     map[string]Tcon     `json:"tcons"`
	OpenFiles map[string]OpenFile `json:"open_files"`
}

type Session struct {
	RemoteMachine string `json:"remote_machine"`
	User          string `json:"user"`
	Protocol      string `json:"protocol_version"`
}

type Tcon struct {
	Service string `json:"service"`
	User    string `json:"user"`
}

type OpenFile struct {
	Path string `json:"path"`
	User string `json:"user"`
}

type SambaUser struct {
	Username string `json:"username"`
	UID      string `json:"uid"`
	FullName string `json:"full_name"`
}

type SambaGroup struct {
	Name    string   `json:"name"`
	GID     string   `json:"gid"`
	Members []string `json:"members"`
}

type ADHealth struct {
	Status     bool            `json:"status"`
	Checks     []ADCheckResult `json:"checks"`
	LastUpdate string          `json:"last_update"`
}

type ADCheckResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "ok", "warning", "error"
	Message string `json:"message"`
}

type AdminUser struct {
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	Hash     string `json:"hash"`
	Role     string `json:"role"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type ShareInfo struct {
	Name         string            `json:"name"`
	Path         string            `json:"path"`
	Comment      string            `json:"comment"`
	IsRecycle    bool              `json:"is_recycle"`
	IsAudit      bool              `json:"is_audit"`
	AuditOpen    bool              `json:"audit_open"`
	IsShadowCopy bool              `json:"is_shadow_copy"`
	Params       map[string]string `json:"params"`
}

type AutomationSettings struct {
	RecycleDays      int    `json:"recycle_days"`
	SnapshotInterval string `json:"snapshot_interval"`
	SnapshotKeep     int    `json:"snapshot_keep"`
}

type GlobalConfig struct {
	Params map[string]string `json:"params"`
}

type DiskUsage struct {
	Path       string   `json:"path"`
	MountPoint string   `json:"mount_point"`
	Total      string   `json:"total"`
	Used       string   `json:"used"`
	Free       string   `json:"free"`
	Percent    float64  `json:"percent"`
	Shares     []string `json:"shares"`
}

type AuditEntry struct {
	Timestamp string `json:"timestamp"`
	User      string `json:"user"`
	IP        string `json:"ip"`
	Action    string `json:"action"`
	File      string `json:"file"`
}

type PathPermissions struct {
	Path  string `json:"path"`
	Owner string `json:"owner"`
	Group string `json:"group"`
	Mode  string `json:"mode"`
	ACLs  string `json:"acls,omitempty"`
}
