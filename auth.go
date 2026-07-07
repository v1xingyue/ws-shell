package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
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
	githubAuthEnabled = false
	passwordUsername  string
	passwordValue     string
)

// GitHub 用户信息结构
type GitHubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
}

// Session 用户信息
type SessionUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Provider string `json:"provider"`
}

func initAuth() {
	// 从环境变量读取配置
	clientID := os.Getenv("GITHUB_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	passwordUsername = strings.TrimSpace(os.Getenv("AUTH_USERNAME"))
	passwordValue = os.Getenv("AUTH_PASSWORD")
	githubAuthEnabled = clientID != "" && clientSecret != ""
	authEnabled = githubAuthEnabled || (passwordUsername != "" && passwordValue != "")

	if !authEnabled {
		logrus.Warn("No auth configured, authentication disabled")
		return
	}

	// 解析允许登录的 GitHub 用户 ID 列表（仅支持 ALLOWED_USER_IDS，见文档获取方法）
	if allowedIDs := os.Getenv("ALLOWED_USER_IDS"); allowedIDs != "" {
		for _, s := range strings.Split(allowedIDs, ",") {
			if s = strings.TrimSpace(s); s != "" {
				allowedUserIDs = append(allowedUserIDs, s)
			}
		}
		logrus.Infof("Allowed user IDs: %v", allowedUserIDs)
	}

	if !githubAuthEnabled {
		logrus.Warn("GITHUB_CLIENT_ID or GITHUB_CLIENT_SECRET not set, GitHub auth disabled")
	}
	if passwordUsername != "" && passwordValue != "" {
		logrus.Infof("Password auth initialized for user %s", passwordUsername)
	}
	if !githubAuthEnabled {
		return
	}

	// 配置 GitHub OAuth
	githubOAuthConfig = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     github.Endpoint,
		RedirectURL:  getRedirectURL(),
		Scopes:       []string{"user:email"},
	}

	logrus.Infof("GitHub OAuth initialized, local dev: %v", localOAuthDevEnabled())
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
	value := strings.TrimSpace(os.Getenv("OAUTH_LOCAL_DEV"))
	return strings.EqualFold(value, "true") || value == "1"
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
		if passwordUsername != "" && passwordValue != "" {
			auth.POST("/password", handlePasswordLogin)
		}
		if githubAuthEnabled {
			auth.GET("/github", handleGitHubLogin)
			auth.GET("/github/callback", handleGitHubCallback)
		}
	}
}

// GitHub 登录处理
func handleGitHubLogin(c *gin.Context) {
	if !githubAuthEnabled || githubOAuthConfig == nil {
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
	if !githubAuthEnabled || githubOAuthConfig == nil {
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

	setSessionCookies(c, SessionUser{
		ID:       fmt.Sprintf("%d", githubUser.ID),
		Username: githubUser.Login,
		Email:    githubUser.Email,
		Provider: "github",
	})

	logrus.Infof("User %s (ID: %d) logged in successfully", githubUser.Login, githubUser.ID)

	// 重定向到前端页面
	c.Redirect(http.StatusFound, "/console")
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

func handlePasswordLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.Username), []byte(passwordUsername)) != 1 ||
		subtle.ConstantTimeCompare([]byte(req.Password), []byte(passwordValue)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	user := SessionUser{
		ID:       "password:" + passwordUsername,
		Username: passwordUsername,
		Provider: "password",
	}
	setSessionCookies(c, user)
	c.JSON(http.StatusOK, user)
}

func setSessionCookies(c *gin.Context, user SessionUser) {
	maxAge := 3600
	c.SetCookie("auth_token", "", -1, "/", "", enableSSL, true)
	c.SetCookie("auth_provider", user.Provider, maxAge, "/", "", enableSSL, true)
	c.SetCookie("user_id", user.ID, maxAge, "/", "", enableSSL, true)
	c.SetCookie("username", user.Username, maxAge, "/", "", enableSSL, true)
	c.SetCookie("auth_session", signSession(user), maxAge, "/", "", enableSSL, true)
}

func clearSessionCookies(c *gin.Context) {
	for _, name := range []string{"auth_token", "auth_provider", "user_id", "username", "auth_session"} {
		c.SetCookie(name, "", -1, "/", "", enableSSL, true)
	}
}

func sessionFromRequest(c *gin.Context) (SessionUser, bool) {
	provider, err := c.Cookie("auth_provider")
	if err != nil {
		return SessionUser{}, false
	}
	userID, err := c.Cookie("user_id")
	if err != nil {
		return SessionUser{}, false
	}
	username, err := c.Cookie("username")
	if err != nil {
		return SessionUser{}, false
	}
	signature, err := c.Cookie("auth_session")
	if err != nil {
		return SessionUser{}, false
	}
	user := SessionUser{ID: userID, Username: username, Provider: provider}
	return user, hmac.Equal([]byte(signature), []byte(signSession(user)))
}

func signSession(user SessionUser) string {
	mac := hmac.New(sha256.New, []byte(sessionSecret()))
	mac.Write([]byte(user.Provider))
	mac.Write([]byte{0})
	mac.Write([]byte(user.ID))
	mac.Write([]byte{0})
	mac.Write([]byte(user.Username))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func sessionSecret() string {
	if secret := os.Getenv("AUTH_SESSION_SECRET"); secret != "" {
		return secret
	}
	return os.Getenv("GITHUB_CLIENT_SECRET") + "\x00" + passwordValue
}

func authMethods() []string {
	methods := []string{}
	if passwordUsername != "" && passwordValue != "" {
		methods = append(methods, "password")
	}
	if githubAuthEnabled {
		methods = append(methods, "github")
	}
	return methods
}

// 登出处理
func handleLogout(c *gin.Context) {
	if !authEnabled {
		// 当认证未启用时，直接重定向到首页
		c.Redirect(http.StatusFound, "/console")
		return
	}

	// 清除 cookie
	clearSessionCookies(c)

	logrus.Info("User logged out")

	// 重定向到登录页面或首页
	c.Redirect(http.StatusFound, "/console")
}

// 获取当前用户信息
func handleMe(c *gin.Context) {
	if !authEnabled {
		// 当认证未启用时，返回默认用户信息
		c.JSON(http.StatusOK, gin.H{
			"id":       "0",
			"username": "guest",
			"provider": "guest",
		})
		return
	}

	user, ok := sessionFromRequest(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":        "Not authenticated",
			"auth_methods": authMethods(),
		})
		return
	}

	c.JSON(http.StatusOK, user)
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
		user, ok := sessionFromRequest(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Not authenticated",
			})
			c.Abort()
			return
		}

		// 检查用户是否在允许列表中（仅 ALLOWED_USER_IDS）
		if user.Provider == "github" && len(allowedUserIDs) > 0 {
			allowed := false
			for _, id := range allowedUserIDs {
				if id == user.ID {
					allowed = true
					break
				}
			}
			if !allowed {
				c.JSON(http.StatusForbidden, gin.H{
					"error":   "User not allowed",
					"user_id": user.ID,
				})
				c.Abort()
				return
			}
		}

		// 将用户信息存储到上下文中
		c.Set("userID", user.ID)
		c.Next()
	}
}
