// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"fmt"
	"log"
	"path"
	"strings"

	"github.com/vmware/govmomi/vapi/library"
)

type Library struct {
	driver  *VCenterDriver
	library *library.Library
}

func (d *VCenterDriver) FindContentLibraryByName(name string) (*Library, error) {
	lm := library.NewManager(d.restClient.client)
	l, err := lm.GetLibraryByName(d.ctx, name)
	if err != nil {
		return nil, err
	}
	return &Library{
		library: l,
		driver:  d,
	}, nil
}

func (d *VCenterDriver) FindContentLibraryItem(libraryId string, name string) (*library.Item, error) {
	lm := library.NewManager(d.restClient.client)
	items, err := lm.GetLibraryItems(d.ctx, libraryId)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Name == name {
			return &item, nil
		}
	}
	return nil, fmt.Errorf("content library item %s not found", name)
}

func (d *VCenterDriver) FindContentLibraryItemUUID(libraryId string, name string) (string, error) {
	item, err := d.FindContentLibraryItem(libraryId, name)
	if err != nil {
		return "", err
	}
	return item.ID, nil
}

func (d *VCenterDriver) FindContentLibraryFileDatastorePath(isoPath string) (string, error) {
	log.Printf("Check if ISO path is a Content Library path")
	err := d.restClient.Login(d.ctx)
	if err != nil {
		log.Printf("vCenter client not available. ISO path not identified as a Content Library path")
		return isoPath, err
	}

	libraryFilePath := &LibraryFilePath{path: isoPath}
	err = libraryFilePath.Validate()
	if err != nil {
		log.Printf("ISO path not identified as a Content Library path")
		return isoPath, err
	}
	libraryName := libraryFilePath.GetLibraryName()
	itemName := libraryFilePath.GetLibraryItemName()
	isoFile := libraryFilePath.GetFileName()

	lib, err := d.FindContentLibraryByName(libraryName)
	if err != nil {
		log.Printf("ISO path not identified as a Content Library path")
		return isoPath, err
	}
	log.Printf("ISO path identified as a Content Library path")
	log.Printf("Finding the equivalent datastore path for the Content Library ISO file path")
	libItem, err := d.FindContentLibraryItem(lib.library.ID, itemName)
	if err != nil {
		log.Printf("[WARN] Couldn't find item %s: %s", itemName, err.Error())
		return isoPath, err
	}
	datastoreName, err := d.GetDatastoreName(lib.library.Storage[0].DatastoreID)
	if err != nil {
		log.Printf("[WARN] Couldn't find datastore name for library %s", libraryName)
		return isoPath, err
	}
	libItemDir := fmt.Sprintf("[%s] contentlib-%s/%s", datastoreName, lib.library.ID, libItem.ID)

	isoFilePath, err := d.GetDatastoreFilePath(lib.library.Storage[0].DatastoreID, libItemDir, isoFile)
	if err != nil {
		log.Printf("[WARN] Couldn't find datastore ID path for %s", isoFile)
		return isoPath, err
	}

	_ = d.restClient.Logout(d.ctx)
	return path.Join(libItemDir, isoFilePath), nil
}

func (d *VCenterDriver) UpdateContentLibraryItem(item *library.Item, name string, description string) error {
	lm := library.NewManager(d.restClient.client)
	item.Patch(&library.Item{
		ID:          item.ID,
		Name:        name,
		Description: &description,
	})
	return lm.UpdateLibraryItem(d.ctx, item)
}

type LibraryFilePath struct {
	path string
}

func (l *LibraryFilePath) Validate() error {
	l.path = strings.TrimLeft(l.path, "/")
	parts := strings.Split(l.path, "/")
	if len(parts) != 3 {
		return fmt.Errorf("content library file path must contain the names for the library, item, and file")
	}
	return nil
}

func (l *LibraryFilePath) GetLibraryName() string {
	return strings.Split(l.path, "/")[0]
}

func (l *LibraryFilePath) GetLibraryItemName() string {
	return strings.Split(l.path, "/")[1]
}

func (l *LibraryFilePath) GetFileName() string {
	return strings.Split(l.path, "/")[2]
}
