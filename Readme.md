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
