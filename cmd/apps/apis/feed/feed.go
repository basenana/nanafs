/*
  Copyright 2023 NanaFS Authors.

  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
*/

package feed

import (
	"encoding/xml"
	"time"
)

type XmlGenerator interface {
	Generate(f Feed) interface{}
}

func ToXML(generator XmlGenerator, feed Feed) (string, error) {
	x := generator.Generate(feed)
	data, err := xml.MarshalIndent(x, "", "  ")
	if err != nil {
		return "", err
	}
	// strip empty line from default xml header
	s := xml.Header[:len(xml.Header)-1] + string(data)
	return s, nil
}

type Feed struct {
	Title       string
	Link        *Link
	Description string
	Author      *Author
	Updated     time.Time
	Created     time.Time
	Id          string
	Items       []*Item
}

type Item struct {
	Title       string
	Link        *Link
	Author      *Author
	Description string // used as description in rss, summary in atom
	Id          string // used as guid in rss, id in atom
	Updated     string
	Created     string
	Content     string
}

type Link struct {
	Href, Rel string
}

type Author struct {
	Name, Email string
}
