package server

import (
	"encoding/json"
	"net/http"
)

type stickerItem struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	URL   string `json:"url"`
	Tone  string `json:"tone"`
}

type stickerPack struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Items       []stickerItem `json:"items"`
}

func lchatStickerPacks() []stickerPack {
	return []stickerPack{
		{
			ID:          "luminous-daily",
			Name:        "星光日常",
			Description: "适合问候、确认、轻松聊天的 Lchat 原生贴图。",
			Items: []stickerItem{
				{ID: "moon-note", Label: "月色便签", URL: "/assets/stickers/moon-note.svg", Tone: "calm"},
				{ID: "ribbon-heart", Label: "丝带心意", URL: "/assets/stickers/ribbon-heart.svg", Tone: "warm"},
				{ID: "tea-break", Label: "茶歇一下", URL: "/assets/stickers/tea-break.svg", Tone: "soft"},
				{ID: "stellar-letter", Label: "星光来信", URL: "/assets/stickers/stellar-letter.svg", Tone: "bright"},
				{ID: "spark-call", Label: "闪光回话", URL: "/assets/stickers/spark-call.svg", Tone: "active"},
				{ID: "lucky-ticket", Label: "幸运票根", URL: "/assets/stickers/lucky-ticket.svg", Tone: "playful"},
				{ID: "rain-window", Label: "雨窗慢聊", URL: "/assets/stickers/rain-window.svg", Tone: "quiet"},
				{ID: "neon-badge", Label: "高光徽章", URL: "/assets/stickers/neon-badge.svg", Tone: "celebrate"},
				{ID: "soft-check", Label: "轻轻确认", URL: "/assets/stickers/soft-check.svg", Tone: "confirm"},
			},
		},
	}
}

func (s *Server) handleLchatMeta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "only GET is supported", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":         "Lchat",
		"tagline":      "连接每一个闪闪发光的瞬间",
		"style":        "二次元日系聊天软件 / 日系游戏 UI / 现代桌面 IM",
		"stickerPacks": lchatStickerPacks(),
	})
}

func (s *Server) handleStickers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "only GET is supported", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"packs": lchatStickerPacks(),
	})
}
