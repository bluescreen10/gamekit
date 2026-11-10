//go:build linux && cgo

package gtk

import "unsafe"

func openNativeURL(url string) bool {
	if url == "" {
		return false
	}
	var err *gError
	opened := gAppInfoLaunchDefaultForURI(stringPointer(url), nil, &err) != 0
	if err != nil {
		gErrorFree(err)
	}
	return opened
}

// fileURI returns an owned native string. GFile handles filename encoding and
// URI escaping, including filenames that are not valid UTF-8.
func fileURI(path string) *cChar {
	file := gFileNewForPath(stringPointer(path))
	defer gObjectUnref(unsafe.Pointer(file))
	return gFileGetURI(file)
}

func revealNativeFile(path string, selectFile bool) bool {
	file := gFileNewForPath(stringPointer(path))
	defer gObjectUnref(unsafe.Pointer(file))
	target := file
	if !selectFile && gFileQueryFileType(file, gFileQueryInfoNone, nil) != gFileTypeDirectory {
		if parent := gFileGetParent(file); parent != nil {
			target = parent
			defer gObjectUnref(unsafe.Pointer(parent))
		}
	}
	uri := gFileGetURI(target)
	defer gFree(unsafe.Pointer(uri))
	var err *gError
	bus := gBusGetSync(gBusTypeSession, nil, &err)
	if bus != nil {
		defer gObjectUnref(unsafe.Pointer(bus))
		method := "ShowFolders"
		if selectFile {
			method = "ShowItems"
		}
		// FileManager1 takes (as, s): a list of URIs and a startup ID. These
		// constructors copy their inputs and adopt floating child variants.
		uriValue := gVariantNewString(uri)
		uris := gVariantNewArray(nil, &uriValue, 1)
		arguments := []*gVariant{uris, gVariantNewString(stringPointer(""))}
		parameters := gVariantNewTuple(unsafe.SliceData(arguments), gSize(len(arguments)))
		reply := gDBusConnectionCallSync(bus,
			stringPointer("org.freedesktop.FileManager1"), stringPointer("/org/freedesktop/FileManager1"),
			stringPointer("org.freedesktop.FileManager1"), stringPointer(method), parameters, nil,
			gDBusCallFlagsNone, 5000, nil, &err)
		if reply != nil {
			gVariantUnref(reply)
			return true
		}
	}
	if err != nil {
		gErrorFree(err)
	}
	return openNativeURL(goString(uri))
}

func recentDocumentPaths() []string {
	items := gtkRecentManagerGetItems(gtkRecentManagerGetDefault())
	defer gListFree(items)
	application := gGetApplicationName()
	var paths []string
	for item := items; item != nil; item = item.next {
		info := (*gtkRecentInfo)(item.data)
		if application != nil && gtkRecentInfoHasApplication(info, application) != 0 {
			file := gFileNewForURI(gtkRecentInfoGetURI(info))
			if path := gFileGetPath(file); path != nil {
				paths = append(paths, takeString(path))
			}
			gObjectUnref(unsafe.Pointer(file))
		}
		gtkRecentInfoUnref(info)
	}
	return paths
}

func clearRecentDocuments() {
	manager := gtkRecentManagerGetDefault()
	items := gtkRecentManagerGetItems(manager)
	defer gListFree(items)
	application := gGetApplicationName()
	for item := items; item != nil; item = item.next {
		info := (*gtkRecentInfo)(item.data)
		if application != nil && gtkRecentInfoHasApplication(info, application) != 0 {
			gtkRecentManagerRemoveItem(manager, gtkRecentInfoGetURI(info), nil)
		}
		gtkRecentInfoUnref(info)
	}
}

func defaultApplication(extension string) string {
	filename := "file"
	if extension[0] != '.' {
		filename += "."
	}
	filename += extension
	contentType := gContentTypeGuess(stringPointer(filename), nil, 0, nil)
	if contentType == nil {
		return ""
	}
	defer gFree(unsafe.Pointer(contentType))
	info := gAppInfoGetDefaultForType(contentType, 0)
	if info == nil {
		return ""
	}
	defer gObjectUnref(unsafe.Pointer(info))
	return goString(gAppInfoGetDisplayName(info))
}
