// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// announcementAttachmentKind says how a file's bytes are recognised, so a file
// cannot claim a type it is not. The names are the families of magic number
// below, not MIME types: several MIME types share one (every Office Open XML
// document is a zip).
type announcementAttachmentKind int

const (
	attachmentKindPDF announcementAttachmentKind = iota + 1
	attachmentKindPNG
	attachmentKindJPEG
	attachmentKindGIF
	attachmentKindOOXML  // docx, xlsx, pptx: a zip container
	attachmentKindLegacy // doc, xls, ppt: an OLE compound file
	attachmentKindText   // txt, csv: UTF-8 text with no NUL byte
)

type announcementAttachmentType struct {
	mime string
	kind announcementAttachmentKind
}

// announcementAttachmentTypes are the file types an announcement may carry,
// by lower-case extension: documents a customer is expected to open (a PDF
// above all) and plain images and text. Anything that can run (archives,
// scripts, executables, HTML) is deliberately absent: a customer downloads
// these files from a notice that comes from WSO2.
// The extension decides the type; the bytes are then checked against it.
var announcementAttachmentTypes = map[string]announcementAttachmentType{
	".pdf":  {"application/pdf", attachmentKindPDF},
	".png":  {"image/png", attachmentKindPNG},
	".jpg":  {"image/jpeg", attachmentKindJPEG},
	".jpeg": {"image/jpeg", attachmentKindJPEG},
	".gif":  {"image/gif", attachmentKindGIF},
	".doc":  {"application/msword", attachmentKindLegacy},
	".docx": {"application/vnd.openxmlformats-officedocument.wordprocessingml.document", attachmentKindOOXML},
	".xls":  {"application/vnd.ms-excel", attachmentKindLegacy},
	".xlsx": {"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", attachmentKindOOXML},
	".ppt":  {"application/vnd.ms-powerpoint", attachmentKindLegacy},
	".pptx": {"application/vnd.openxmlformats-officedocument.presentationml.presentation", attachmentKindOOXML},
	".txt":  {"text/plain", attachmentKindText},
	".csv":  {"text/csv", attachmentKindText},
}

// announcementAttachmentAllowedExtensions lists the accepted extensions for an
// error message, in a stable order.
const announcementAttachmentAllowedExtensions = "pdf, png, jpg, jpeg, gif, doc, docx, xls, xlsx, ppt, pptx, txt, csv"

// maxAnnouncementAttachmentNameLength is the longest file name stored.
const maxAnnouncementAttachmentNameLength = 255

// validateAnnouncementAttachmentName returns the trimmed name when it is a
// plain file name with an extension this feature accepts.
func validateAnnouncementAttachmentName(raw string) (string, announcementAttachmentType, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", announcementAttachmentType{}, &apierror.ValidationError{Msg: "name is required"}
	}
	if utf8.RuneCountInString(name) > maxAnnouncementAttachmentNameLength {
		return "", announcementAttachmentType{}, &apierror.ValidationError{Msg: fmt.Sprintf("name must not exceed %d characters", maxAnnouncementAttachmentNameLength)}
	}
	// A name is stored and shown verbatim and becomes the file name a customer
	// saves, so it must not carry a path or control characters.
	if strings.ContainsAny(name, `/\`) {
		return "", announcementAttachmentType{}, &apierror.ValidationError{Msg: "name must be a file name, without a path"}
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", announcementAttachmentType{}, &apierror.ValidationError{Msg: "name must not contain control characters"}
		}
	}
	ext := strings.ToLower(path.Ext(name))
	t, ok := announcementAttachmentTypes[ext]
	if !ok {
		return "", announcementAttachmentType{}, &apierror.ValidationError{Msg: fmt.Sprintf("%q is not a supported file type: an announcement may carry %s files", name, announcementAttachmentAllowedExtensions)}
	}
	return name, t, nil
}

// decodeAnnouncementAttachmentFile reads the base64 data URI the request
// carries and returns the decoded bytes. It refuses an oversize file before
// decoding it (the decoded size of base64 is about three quarters of its length).
func decodeAnnouncementAttachmentFile(file string) ([]byte, error) {
	const dataURIPrefix = "data:"
	const base64Marker = ";base64,"
	if file == "" {
		return nil, &apierror.ValidationError{Msg: "file is required"}
	}
	if !strings.HasPrefix(file, dataURIPrefix) {
		return nil, &apierror.ValidationError{Msg: "file must be a base64 data URI (e.g. data:application/pdf;base64,...)"}
	}
	markerIdx := strings.Index(file, base64Marker)
	if markerIdx == -1 {
		return nil, &apierror.ValidationError{Msg: "file must be a base64 data URI (e.g. data:application/pdf;base64,...)"}
	}
	raw := file[markerIdx+len(base64Marker):]
	if len(raw)*3/4 > domain.MaxAnnouncementRequestAttachmentBytes+3 {
		return nil, &apierror.ValidationError{Msg: "file exceeds the maximum allowed size of 10 MB"}
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(raw)
		if err != nil {
			return nil, &apierror.ValidationError{Msg: "file contains invalid base64 data"}
		}
	}
	if len(decoded) == 0 {
		return nil, &apierror.ValidationError{Msg: "file is empty"}
	}
	if len(decoded) > domain.MaxAnnouncementRequestAttachmentBytes {
		return nil, &apierror.ValidationError{Msg: "file exceeds the maximum allowed size of 10 MB"}
	}
	return decoded, nil
}

// checkAnnouncementAttachmentContent refuses bytes that are not the kind of
// file the name says they are. The name's extension and the declared MIME type
// are both the uploader's word; this is the one check that looks at the file.
func checkAnnouncementAttachmentContent(name string, kind announcementAttachmentKind, content []byte) error {
	ok := false
	switch kind {
	case attachmentKindPDF:
		// A PDF may be preceded by a few bytes of junk, but the header is
		// within the first kilobyte (PDF 32000-1, Annex H).
		head := content
		if len(head) > 1024 {
			head = head[:1024]
		}
		ok = bytes.Contains(head, []byte("%PDF-"))
	case attachmentKindPNG:
		ok = bytes.HasPrefix(content, []byte("\x89PNG\r\n\x1a\n"))
	case attachmentKindJPEG:
		ok = bytes.HasPrefix(content, []byte{0xFF, 0xD8, 0xFF})
	case attachmentKindGIF:
		ok = bytes.HasPrefix(content, []byte("GIF87a")) || bytes.HasPrefix(content, []byte("GIF89a"))
	case attachmentKindOOXML:
		ok = bytes.HasPrefix(content, []byte("PK\x03\x04"))
	case attachmentKindLegacy:
		ok = bytes.HasPrefix(content, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
	case attachmentKindText:
		ok = utf8.Valid(content) && !bytes.Contains(content, []byte{0})
	}
	if !ok {
		return &apierror.ValidationError{Msg: fmt.Sprintf("%q is not a valid %s file: its content does not match its type", name, strings.TrimPrefix(strings.ToLower(path.Ext(name)), "."))}
	}
	return nil
}

// buildAnnouncementAttachmentDataURI is the inverse of
// decodeAnnouncementAttachmentFile: the data URI the case attachment path takes.
func buildAnnouncementAttachmentDataURI(mime string, content []byte) string {
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(content)
}
