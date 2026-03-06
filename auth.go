package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	allowedIDs := os.Getenv("ALLOWED_USER_IDS")

	if clientID == "" || clientSecret == "" {
		logrus.Warn("GITHUB_CLIENT_ID or GITHUB_CLIENT_SECRET not set, GitHub auth disabled")
		return
	}

	// 解析允许的用户 ID
	if allowedIDs != "" {
		allowedUserIDs = strings.Split(allowedIDs, ",")
		for i, id := range allowedUserIDs {
			allowedUserIDs[i] = strings.TrimSpace(id)
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
	// 从环境变量获取，或者使用默认值
	redirectURL := os.Getenv("OAUTH_REDIRECT_URL")
	if redirectURL == "" {
		// 默认使用当前服务器的地址
		if enableSSL {
			redirectURL = fmt.Sprintf("https://%s/auth/github/callback", bindAddress)
		} else {
			redirectURL = fmt.Sprintf("http://%s/auth/github/callback", bindAddress)
		}
	}
	return redirectURL
}

// GitHub 认证路由
func setupAuthRoutes(r *gin.Engine) {
	auth := r.Group("/auth")
	{
		auth.GET("/github", handleGitHubLogin)
		auth.GET("/github/callback", handleGitHubCallback)
		auth.GET("/logout", handleLogout)
		auth.GET("/me", handleMe)
	}
}

// GitHub 登录处理
func handleGitHubLogin(c *gin.Context) {
	if githubOAuthConfig == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "GitHub auth not configured",
		})
		return
	}

	// 生成状态字符串（在生产环境中应该使用更安全的方式）
	state := oauthStateString
	url := githubOAuthConfig.AuthCodeURL(state)
	c.Redirect(http.StatusFound, url)
}

// GitHub 回调处理
func handleGitHubCallback(c *gin.Context) {
	if githubOAuthConfig == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "GitHub auth not configured",
		})
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

	// 交换授权码获取访问令牌
	token, err := githubOAuthConfig.Exchange(context.Background(), code)
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
			"error": "User not allowed",
			"user":  githubUser.Login,
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

// 检查用户是否在允许列表中
func isUserAllowed(user GitHubUser) bool {
	// 如果没有配置允许列表，允许所有用户
	if len(allowedUserIDs) == 0 {
		return true
	}

	userIDStr := fmt.Sprintf("%d", user.ID)
	for _, allowedID := range allowedUserIDs {
		if allowedID == userIDStr {
			return true
		}
	}

	return false
}

// 登出处理
func handleLogout(c *gin.Context) {
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
		// 检查 cookie 中的认证信息
		userID, err := c.Cookie("user_id")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Not authenticated",
			})
			c.Abort()
			return
		}

		// 检查用户是否在允许列表中
		if len(allowedUserIDs) > 0 {
			allowed := false
			for _, allowedID := range allowedUserIDs {
				if allowedID == userID {
					allowed = true
					break
				}
			}
			if !allowed {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "User not allowed",
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
