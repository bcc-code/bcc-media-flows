package utils

import (
	"os"
	"path/filepath"
	"strings"
)

func GetSiblingFolder(path, folder string) (string, error) {
	newFolder := filepath.Clean(filepath.Join(filepath.Dir(path), "..", folder))
	err := os.MkdirAll(newFolder, os.ModePerm)
	if err != nil {
		return "", err
	}
	newFolder, err = filepath.Abs(newFolder)
	if err != nil {
		return "", err
	}
	return newFolder, nil
}

func MoveToParentFolder(path, folder string) (string, error) {
	filename := filepath.Base(path)
	newFolder, err := GetSiblingFolder(path, folder)
	newPath := filepath.Join(newFolder, filename)
	err = os.Rename(path, newPath)
	if err != nil {
		return "", err
	}
	return newPath, nil
}

func FixFilename(path string) (string, error) {
	filename := filepath.Base(path)
	newFilename := strings.Replace(filepath.Clean(filename), " ", "_", -1)
	newPath := filepath.Join(filepath.Dir(path), newFilename)
	err := os.Rename(path, newPath)
	if err != nil {
		return "", err
	}
	return newPath, nil
}
