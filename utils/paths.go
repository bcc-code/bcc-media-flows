package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

func GetSiblingFolder(path, folder string) string {
	newFolder := filepath.Clean(filepath.Join(filepath.Dir(path), "..", folder))
	err := os.MkdirAll(newFolder, os.ModePerm)
	if err != nil {
		fmt.Println(err)
	}
	return newFolder
}

func MoveToSiblingFolder(path, folder string) string {
	filename := filepath.Base(path)
	newFolder := GetSiblingFolder(path, folder)
	newPath := filepath.Join(newFolder, filename)
	err := os.Rename(path, newPath)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	return newPath
}
