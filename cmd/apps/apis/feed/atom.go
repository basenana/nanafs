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
	"path"
	"time"
)

const ns = "http://www.w3.org/2005/Atom"

type AtomXmlGenerator struct{}

func NewAtomGenerator() XmlGenerator {
	return &AtomXmlGenerator{}
}

func (a *AtomXmlGenerator) Generate(f Feed) interface{} {
	links := []AtomLink{}
	if f.Link != nil {
		links = append(links, AtomLink{Href: f.Link.Href, Rel: f.Link.Rel})
		links = append(links, AtomLink{
			Href: path.Join(f.Link.Href, "/atom.xml"),
			Rel:  "self",
		})
	}
	author := AtomAuthor{}
	if f.Author != nil {
		author = AtomAuthor{
			AtomPerson: AtomPerson{
				Name:  f.Author.Name,
				Email: f.Author.Email,
			},
		}
	}
	entries := []*AtomEntry{}
	for _, item := range f.Items {
		link := AtomLink{}
		if item.Link != nil {
			link = AtomLink{
				Href: item.Link.Href,
				Rel:  item.Link.Rel,
			}
		}
		entries = append(entries, &AtomEntry{
			Title:   item.Title,
			Links:   []AtomLink{link},
			Updated: item.Updated.Format(time.RFC3339),
			Id:      item.Id,
			Content: &AtomContent{
				Content: item.Content,
				Type:    "html",
			},
			Summary: &AtomSummary{
				XMLName: xml.Name{},
				Content: item.Description,
				Type:    "html",
			},
		})
	}
	af := &AtomFeed{
		Xmlns:   ns,
		Title:   f.Title,
		Link:    links,
		Updated: f.Updated.Format(time.RFC3339),
		Id:      f.Id,
		Author:  &author,
		Entries: entries,
	}
	return af
}

type AtomFeed struct {
	XMLName xml.Name `xml:"feed"`
	Xmlns   string   `xml:"xmlns,attr"`

	Title   string `xml:"title"` // required
	Link    []AtomLink
	Updated string       `xml:"updated"` // required
	Id      string       `xml:"id"`      // required
	Author  *AtomAuthor  `xml:"author,omitempty"`
	Entries []*AtomEntry `xml:"entry"`
}

type AtomLink struct {
	//Atom 1.0 <link rel="enclosure" title="MP3" href="http://www.example.org/myaudiofile.mp3" />
	XMLName xml.Name `xml:"link"`
	Href    string   `xml:"href,attr"`
	Rel     string   `xml:"rel,attr,omitempty"`
}

type AtomAuthor struct {
	XMLName xml.Name `xml:"author"`
	AtomPerson
}

type AtomPerson struct {
	Name  string `xml:"name,omitempty"`
	Email string `xml:"email,omitempty"`
}

type AtomEntry struct {
	XMLName xml.Name `xml:"entry"`
	Xmlns   string   `xml:"xmlns,attr,omitempty"`

	Title   string `xml:"title"` // required
	Links   []AtomLink
	Updated string `xml:"updated"` // required
	Id      string `xml:"id"`      // required
	Content *AtomContent
	Summary *AtomSummary // required if content has src or content is base64
}

type AtomSummary struct {
	XMLName xml.Name `xml:"summary"`
	Content string   `xml:",chardata"`
	Type    string   `xml:"type,attr"`
}

type AtomContent struct {
	XMLName xml.Name `xml:"content"`
	Content string   `xml:",chardata"`
	Type    string   `xml:"type,attr"`
}
