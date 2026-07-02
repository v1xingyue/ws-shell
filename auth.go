package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

// GitHub OAuth 配置
var (
	githubOAuthConfig *oauth2.Config
	allowedUserIDs    []string
	oauthStateString  = "random-state-string" // 在生产环境中应该随机生成
	authEnabled       = false
)

// GitHub 用户信息结构
type GitHubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
}

// Session 用户信息
type SessionUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

func initAuth() {
	// 从环境变量读取配置
	clientID := os.Getenv("GITHUB_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		logrus.Warn("GITHUB_CLIENT_ID or GITHUB_CLIENT_SECRET not set, GitHub auth disabled")
		authEnabled = false
		return
	}

	authEnabled = true

	// 解析允许登录的 GitHub 用户 ID 列表（仅支持 ALLOWED_USER_IDS，见文档获取方法）
	if allowedIDs := os.Getenv("ALLOWED_USER_IDS"); allowedIDs != "" {
		for _, s := range strings.Split(allowedIDs, ",") {
			if s = strings.TrimSpace(s); s != "" {
				allowedUserIDs = append(allowedUserIDs, s)
			}
		}
		logrus.Infof("Allowed user IDs: %v", allowedUserIDs)
	}

	// 配置 GitHub OAuth
	githubOAuthConfig = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     github.Endpoint,
		RedirectURL:  getRedirectURL(),
		Scopes:       []string{"user:email"},
	}

	logrus.Info("GitHub OAuth initialized")
}

func getRedirectURL() string {
	// 仅作 init 时 Config 的默认值，实际请求中用 redirectURLFromRequest 按请求 Host 自动拼
	redirectURL := os.Getenv("OAUTH_REDIRECT_URL")
	if redirectURL != "" {
		return redirectURL
	}
	host := bindAddress
	if strings.HasPrefix(bindAddress, ":") {
		host = getDefaultIP() + bindAddress
	}
	if enableSSL {
		return fmt.Sprintf("https://%s/auth/github/callback", host)
	}
	return fmt.Sprintf("http://%s/auth/github/callback", host)
}

// redirectURLFromRequest 按当前请求的 Host 和协议自动拼回调地址，无需在 .env 配置 OAUTH_REDIRECT_URL
func redirectURLFromRequest(c *gin.Context) string {
	if redirectURL := os.Getenv("OAUTH_REDIRECT_URL"); redirectURL != "" {
		return redirectURL
	}
	if localOAuthDevEnabled() || c.Query("local") == "true" {
		return localOAuthRedirectURL()
	}
	scheme := "http"
	if c.GetHeader("X-Forwarded-Proto") == "https" || c.Request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host + "/auth/github/callback"
}

func localOAuthDevEnabled() bool {
	return os.Getenv("OAUTH_LOCAL_DEV") == "true"
}

func localOAuthRedirectURL() string {
	if redirectURL := os.Getenv("OAUTH_LOCAL_REDIRECT_URL"); redirectURL != "" {
		return redirectURL
	}
	return "https://ws-shell.vercel.app/auth/github/callback?local=true"
}

func localOAuthCallbackURL() string {
	if callbackURL := os.Getenv("OAUTH_LOCAL_CALLBACK_URL"); callbackURL != "" {
		return callbackURL
	}
	return "http://localhost/auth/github/callback"
}

func redirectLocalOAuthCallback(c *gin.Context) bool {
	if c.Query("local") != "true" || localOAuthDevEnabled() {
		return false
	}
	callbackURL, err := url.Parse(localOAuthCallbackURL())
	if err != nil || callbackURL.Scheme == "" || callbackURL.Host == "" {
		logrus.Errorf("Invalid OAUTH_LOCAL_CALLBACK_URL: %s", localOAuthCallbackURL())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid local OAuth callback URL",
		})
		return true
	}
	query := callbackURL.Query()
	for _, key := range []string{"code", "state", "error", "error_description"} {
		if value := c.Query(key); value != "" {
			query.Set(key, value)
		}
	}
	query.Set("local", "true")
	callbackURL.RawQuery = query.Encode()
	c.Redirect(http.StatusFound, callbackURL.String())
	return true
}

// GitHub 认证路由
// 无论认证是否启用，都注册 /auth/me 和 /auth/logout，以便前端在未配置时能拿到 guest 状态并跳过登录页
func setupAuthRoutes(r *gin.Engine) {
	auth := r.Group("/auth")
	{
		auth.GET("/me", handleMe)
		auth.GET("/logout", handleLogout)
		if authEnabled {
			auth.GET("/github", handleGitHubLogin)
			auth.GET("/github/callback", handleGitHubCallback)
		}
	}
}

// GitHub 登录处理
func handleGitHubLogin(c *gin.Context) {
	if !authEnabled || githubOAuthConfig == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "GitHub auth not configured",
		})
		return
	}

	// 按请求 Host 自动拼回调地址，与 GitHub 里填的 Authorization callback URL 一致即可
	redirectURL := redirectURLFromRequest(c)
	logrus.Infof("GitHub OAuth redirect URL: %s", redirectURL)
	state := oauthStateString
	url := githubOAuthConfig.AuthCodeURL(state, oauth2.SetAuthURLParam("redirect_uri", redirectURL))
	c.Redirect(http.StatusFound, url)
}

// GitHub 回调处理
func handleGitHubCallback(c *gin.Context) {
	if !authEnabled || githubOAuthConfig == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "GitHub auth not configured",
		})
		return
	}
	if redirectLocalOAuthCallback(c) {
		return
	}

	// 检查状态参数
	state := c.Query("state")
	if state != oauthStateString {
		logrus.Warn("Invalid OAuth state")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid state parameter",
		})
		return
	}

	// 获取授权码
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No authorization code provided",
		})
		return
	}

	// 交换授权码获取访问令牌（用与跳转时一致的回调地址）
	redirectURL := redirectURLFromRequest(c)
	config := *githubOAuthConfig
	config.RedirectURL = redirectURL
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		logrus.Errorf("Failed to exchange token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to exchange token",
		})
		return
	}

	// 使用访问令牌获取用户信息
	client := githubOAuthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		logrus.Errorf("Failed to get user info: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get user info",
		})
		return
	}
	defer resp.Body.Close()

	// 解析用户信息
	var githubUser GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&githubUser); err != nil {
		logrus.Errorf("Failed to parse user info: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to parse user info",
		})
		return
	}

	// 检查用户是否在允许列表中
	if !isUserAllowed(githubUser) {
		logrus.Warnf("User %s (ID: %d) not allowed", githubUser.Login, githubUser.ID)
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "User not allowed",
			"user":    githubUser.Login,
			"user_id": githubUser.ID,
		})
		return
	}

	// 设置 cookie
	c.SetCookie("auth_token", token.AccessToken, 3600, "/", "", enableSSL, true)
	c.SetCookie("user_id", fmt.Sprintf("%d", githubUser.ID), 3600, "/", "", enableSSL, true)
	c.SetCookie("username", githubUser.Login, 3600, "/", "", enableSSL, true)

	logrus.Infof("User %s (ID: %d) logged in successfully", githubUser.Login, githubUser.ID)

	// 重定向到前端页面
	c.Redirect(http.StatusFound, "/web")
}

// 检查用户是否在允许列表中（仅按 GitHub 用户 ID）
func isUserAllowed(user GitHubUser) bool {
	if len(allowedUserIDs) == 0 {
		return true
	}
	userIDStr := fmt.Sprintf("%d", user.ID)
	for _, id := range allowedUserIDs {
		if id == userIDStr {
			return true
		}
	}
	return false
}

// 登出处理
func handleLogout(c *gin.Context) {
	if !authEnabled {
		// 当认证未启用时，直接重定向到首页
		c.Redirect(http.StatusFound, "/web")
		return
	}

	// 清除 cookie
	c.SetCookie("auth_token", "", -1, "/", "", enableSSL, true)
	c.SetCookie("user_id", "", -1, "/", "", enableSSL, true)
	c.SetCookie("username", "", -1, "/", "", enableSSL, true)

	logrus.Info("User logged out")

	// 重定向到登录页面或首页
	c.Redirect(http.StatusFound, "/web")
}

// 获取当前用户信息
func handleMe(c *gin.Context) {
	if !authEnabled {
		// 当认证未启用时，返回默认用户信息
		c.JSON(http.StatusOK, gin.H{
			"id":       "0",
			"username": "guest",
		})
		return
	}

	userID, err := c.Cookie("user_id")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Not authenticated",
		})
		return
	}

	username, _ := c.Cookie("username")

	c.JSON(http.StatusOK, gin.H{
		"id":       userID,
		"username": username,
	})
}

// 认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果认证未启用，直接放行
		if !authEnabled {
			c.Next()
			return
		}

		// 检查 cookie 中的认证信息
		userID, err := c.Cookie("user_id")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Not authenticated",
			})
			c.Abort()
			return
		}

		// 检查用户是否在允许列表中（仅 ALLOWED_USER_IDS）
		if len(allowedUserIDs) > 0 {
			allowed := false
			for _, id := range allowedUserIDs {
				if id == userID {
					allowed = true
					break
				}
			}
			if !allowed {
				c.JSON(http.StatusForbidden, gin.H{
					"error":   "User not allowed",
					"user_id": userID,
				})
				c.Abort()
				return
			}
		}

		// 将用户信息存储到上下文中
		c.Set("userID", userID)
		c.Next()
	}
}
