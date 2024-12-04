package main

import "os"

// create .bashrc file to set alias and color
var bashRcContent = `
# enable color support
alias ls='ls --color=auto'
alias grep='grep --color=auto'
alias dir='dir --color=auto'
alias vdir='vdir --color=auto'
alias diff='diff --color=auto'
alias ip='ip --color=auto'
# update PS1 prompt with color
export PS1='\[\033[01;32m\]\u@\h\[\033[00m\]:\[\033[01;34m\]\w\[\033[00m\]\$ '

`
var autoEnv = []string{
	"TERM=xterm-256color",
	"COLORTERM=truecolor",
	"FORCE_COLOR=true",
	"LANG=en_US.UTF-8",
	"LC_ALL=en_US.UTF-8",
	"CLICOLOR=1",
	"CLICOLOR_FORCE=1",
	"LS_COLORS=rs=0:di=01;34:ln=01;36:mh=00:pi=40;33:so=01;35:do=01;35:bd=40;33;01:cd=40;33;01:or=40;31;01:mi=00:su=37;41:sg=30;43:ca=30;41:tw=30;42:ow=34;42:st=37;44:ex=01;32",
}

func getDefualtUser() string {
	u := os.Getenv("USER")
	if u == "" {
		u = "root"
	}
	return u
}

var defaultUser = getDefualtUser()

// var defaultCommand = []string{"/bin/bash", "-l"}

var singleUserMode = true
var webDistPrefix = "web/dist/"
var debugMode = false
var bindAddress = ":8080"
var forkCmd = "/bin/bash"
