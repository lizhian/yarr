package auth

import (
	"net/http"
	"strings"

	"github.com/nkanaev/yarr/src/assets"
	"github.com/nkanaev/yarr/src/server/router"
	"github.com/nkanaev/yarr/src/storage"
)

type Middleware struct {
	BasePath string
	Public   []string
	DB       *storage.Storage
}

func unsafeMethod(method string) bool {
	return method == "POST" || method == "PUT" || method == "DELETE"
}

func (m *Middleware) Handler(c *router.Context) {
	authConfig := m.DB.GetAuthConfig()
	if !authConfig.Enabled {
		c.Next()
		return
	}
	for _, path := range m.Public {
		if strings.HasPrefix(c.Req.URL.Path, m.BasePath+path) {
			c.Next()
			return
		}
	}
	if IsAuthenticated(c.Req, authConfig.Username, authConfig.Password) {
		c.Next()
		return
	}

	rootUrl := m.BasePath + "/"

	if c.Req.URL.Path != rootUrl {
		c.Out.WriteHeader(http.StatusUnauthorized)
		return
	}

	if c.Req.Method == "POST" {
		username := c.Req.FormValue("username")
		password := c.Req.FormValue("password")
		if StringsEqual(username, authConfig.Username) && StringsEqual(password, authConfig.Password) {
			Authenticate(c.Out, authConfig.Username, authConfig.Password, m.BasePath)
			c.Redirect(rootUrl)
			return
		} else {
			c.HTML(http.StatusOK, assets.Template("login.html"), map[string]interface{}{
				"username": username,
				"error":    "用户名或密码错误",
				"settings": m.DB.GetSettings(),
			})
			return
		}
	}
	c.HTML(http.StatusOK, assets.Template("login.html"), map[string]interface{}{
		"settings": m.DB.GetSettings(),
	})
}
