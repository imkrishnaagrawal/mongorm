# Mongorm

Mongorm is a lightweight ORM (Object-Relational Mapping) package for MongoDB, designed for Go applications. This project represents an attempt to build an ORM that mirrors the ease of use and functionality of Gorm, but with a specific focus on MongoDB's document-oriented nature.

## Project Background

Mongorm was initially developed for my personal side project as a way to simplify interactions with MongoDB using Go. The project is born out of a desire to create a tool that combines the flexibility of MongoDB with the simplicity and elegance of Gorm's interface.

**This project is currently in an experimental phase** and serves as a platform for exploration and learning in both MongoDB and ORM design. While Mongorm is functional for basic use cases, it is actively being developed, and its APIs are subject to change.

## Open to Contributors

Mongorm is an open-source project, and contributions are warmly welcomed. Whether you are looking to fix bugs, add new features, or simply suggest improvements, your input is valuable. This project offers a great opportunity for anyone interested in contributing to an open-source project, learning more about ORM systems, or exploring MongoDB's capabilities with Go.

If you've found Mongorm useful for your projects or are interested in contributing to its development, I encourage you to get involved. By collaborating, we can make Mongorm a robust and user-friendly ORM for MongoDB that serves the needs of Go developers in various projects.

## Features

- Easy-to-use API for CRUD operations
- Custom BSON marshaling/unmarshaling support
- Query builder for crafting complex queries
- Support for MongoDB aggregation pipelines
- Efficient handling of MongoDB connections and contexts

## Installation

```sh
go get github.com/imkrishnaagrawal/mongorm

```
## Examples


```go
package config

import (
	"context"
	"log"

	"github.com/imkrishnaagrawal/mongorm"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var MORM *mongorm.MongoORM

func Connect() {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb://rootuser:rootpass@localhost:27017"))
	if err != nil {
		log.Fatal(err)
	}

	MORM = mongorm.NewMongoORM(
		client,
		"testDb",
	)

}

```

```go
package models

import (
	"github.com/imkrishnaagrawal/mongorm"
)

type User struct {
	mongorm.OrmModel `bson:",inline"`
	Username         string `json:"username" bson:"username,omitempty"`
	Email            string `json:"email" bson:"email,omitempty"`
	FullName         string `json:"full_name" bson:"full_name,omitempty"`
	PasswordHash     string `json:"-" bson:"password_hash,omitempty"` // Excluded from JSON responses
}
```

```go
package controllers

import (
	"context"
	"net/http"
	"yourproject/models" // Adjust the import path according to your project structure
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Assuming config.DB is a *mongo.Client instance

func GetUser(c *gin.Context) {
	var user models.User

	if err := config.MORM.First(&user, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	}

	c.JSON(http.StatusOK, &user)
}

func GetUsers(c *gin.Context) {
	var users []models.User

	if err := config.MORM.Find(&users).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	}

	c.JSON(http.StatusOK, &users)
}

func CreateUser(c *gin.Context) {
	var user models.User
	if err := c.BindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := config.MORM.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, &user)
}

func UpdateUser(c *gin.Context) {
	var user models.User

	if err := config.MORM.Where("id = ?", c.Param("id")).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if err := c.BindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := config.MORM.Save(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

func DeleteUser(c *gin.Context) {
	var user models.User

	if result := config.MORM.Where("id = ?", c.Param("id")).Delete(&user); result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
}

func UserRouter(router *gin.Engine) {
	router.GET("/users/:id", GetUser)
	router.GET("/users", GetUsers)
	router.POST("/users", CreateUser)
	router.PATCH("/users/:id", UpdateUser)
	router.DELETE("/users/:id", DeleteUser)
}
```
