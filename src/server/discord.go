package server

import (
	"net/http"
	"strconv"

	"github.com/nkanaev/yarr/src/server/router"
)

func (s *Server) handleDiscordRSS(c *router.Context) {
	c.Out.Header().Set("Cache-Control", "no-store")
	if c.Req.Method != http.MethodGet {
		c.Out.Header().Set("Allow", "GET")
		c.Out.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.discord == nil {
		http.Error(c.Out, "Discord RSS 暂时不可用。", http.StatusServiceUnavailable)
		return
	}
	result := s.discord.Get(c.Req.Context(), c.Vars["channelId"], c.Vars["token"])
	if result.Status == http.StatusOK {
		c.Out.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	} else {
		c.Out.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	if result.RetryAfter > 0 {
		c.Out.Header().Set("Retry-After", strconv.Itoa(result.RetryAfter))
	}
	c.Out.WriteHeader(result.Status)
	c.Out.Write(result.Body)
}
