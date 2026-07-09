//go:build !embedui

package main

import "github.com/gin-gonic/gin"

// attachSPA is a no-op unless the binary is built with `-tags embedui`.
func attachSPA(_ *gin.Engine) {}
