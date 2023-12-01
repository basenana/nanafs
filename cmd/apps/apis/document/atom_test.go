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

package document

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TestAtomXmlGenerator", func() {
	var (
		atomGenerator XmlGenerator
	)

	BeforeEach(func() {
		atomGenerator = NewAtomGenerator()
	})

	Context("generate atom xml", func() {
		It("should be succeed", func() {
			updatedStr := "2023-11-11T21:34:50+08:00"
			updated, _ := time.Parse(time.RFC3339, updatedStr)

			f := Feed{
				Title:       "hdls",
				Link:        &Link{Href: "https://blog.hdls.me/"},
				Description: "blog of hdls",
				Author:      &Author{Name: "hdls"},
				Updated:     updated,
				Id:          "https://blog.hdls.me/",
				Generator: &Generator{
					Value: "NanaFS",
					URI:   "https://github.com/basenana/nanafs",
				},
				Items: []*Item{
					{
						Title:       "artitle A",
						Link:        &Link{Href: "https://blog.hdls.me/a.html"},
						Description: "summary for A",
						Id:          "https://blog.hdls.me/a.html",
						Updated:     updatedStr,
						Content:     "content for A",
					},
					{
						Title:       "artitle B",
						Link:        &Link{Href: "https://blog.hdls.me/b.html"},
						Description: "summary for B",
						Id:          "https://blog.hdls.me/b.html",
						Updated:     updatedStr,
						Content:     "content for B",
					},
				},
			}
			x, err := ToXML(atomGenerator, f)
			fmt.Println(x)
			Expect(err).Should(BeNil())
			Expect(x).Should(Equal(`<?xml version="1.0" encoding="UTF-8"?><feed xmlns="http://www.w3.org/2005/Atom">
  <title>hdls</title>
  <link href="https://blog.hdls.me/"></link>
  <updated>2023-11-11T21:34:50+08:00</updated>
  <id>https://blog.hdls.me/</id>
  <author>
    <name>hdls</name>
  </author>
  <generator uri="https://github.com/basenana/nanafs">NanaFS</generator>
  <entry>
    <title>artitle A</title>
    <link href="https://blog.hdls.me/a.html"></link>
    <updated>2023-11-11T21:34:50+08:00</updated>
    <id>https://blog.hdls.me/a.html</id>
    <content type="html">content for A</content>
    <summary type="html">summary for A</summary>
  </entry>
  <entry>
    <title>artitle B</title>
    <link href="https://blog.hdls.me/b.html"></link>
    <updated>2023-11-11T21:34:50+08:00</updated>
    <id>https://blog.hdls.me/b.html</id>
    <content type="html">content for B</content>
    <summary type="html">summary for B</summary>
  </entry>
</feed>`))
			fmt.Println(x)
		})
	})
})
