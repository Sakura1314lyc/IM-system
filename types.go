package main

// UserProfile 用户全面信息
type UserProfile struct {
	Name      string `json:"name"`
	Avatar    string `json:"avatar"`
	Status    string `json:"status"`    // online, away, offline
	CreatedAt int64  `json:"createdAt"` // 创建时间戳
}

// UserCred 改进后的用户凭据（支持头像）
type UserCredImproved struct {
	Password string `json:"password"`
	Avatar   string `json:"avatar"` // 头像URL或Base64
}
