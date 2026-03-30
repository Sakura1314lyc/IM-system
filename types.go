package main

import (
	"time"
)

// Message 统一的消息格式
type Message struct {
	Type      string    `json:"type"`      // system, text, private, group, status
	From      string    `json:"from"`      // 发送者
	To        string    `json:"to"`        // 私聊对象或群名
	Content   string    `json:"content"`   // 消息内容
	Avatar    string    `json:"avatar"`    // 发送者头像
	Timestamp time.Time `json:"timestamp"` // 时间戳
	GroupName string    `json:"group_name"`// 群聊名
	Mode      string    `json:"mode"`      // public, private, group
}

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

// Group 群组信息
type Group struct {
	Name        string    `json:"name"`
	Creator     string    `json:"creator"`
	Members     []string  `json:"members"`
	CreatedAt   time.Time `json:"createdAt"`
	Description string    `json:"description"`
}
