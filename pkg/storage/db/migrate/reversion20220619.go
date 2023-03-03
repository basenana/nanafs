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

package migrate

const (
	reversion20220619 = "20220619.1"
)

const reversion20220619Upgrade = `
CREATE TABLE object_workflow (
    id VARCHAR(32),
	synced boolean,
	created_at DATETIME,
	updated_at DATETIME,
	PRIMARY KEY (id)
);
`
const reversion20220619Downgrade = `
DROP TABLE object_workflow;
`
