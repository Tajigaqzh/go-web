# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go web application (Go 1.26) using Gin framework, GORM ORM with MySQL, and testify for testing. Provides REST API for user management.

## Development Commands

- `go run main.go` - Start server on :8080
- `go test -v` - Run all tests with verbose output
- `go test -run TestName` - Run specific test
- `go build` - Build executable

## Architecture

Single-file application (main.go) with:
- Gin router handling REST endpoints (/ping, /users GET/POST)
- GORM managing User model and MySQL database
- HTTP handlers inline in main()

Tests in main_test.go use in-memory SQLite for isolation.

## Database Configuration

MySQL connection string in main.go (dsn variable):
- Default: `root:password@tcp(127.0.0.1:3306)/test`
- Modify username, password, host, and database name as needed

## Dependencies

- github.com/gin-gonic/gin - Web framework
- gorm.io/gorm + gorm.io/driver/mysql - ORM and MySQL driver
- github.com/stretchr/testify - Testing assertions

## IDE Integration

JetBrains GoLand project files are tracked (.idea/, go-web.iml).
