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

package metrics

import (
	"github.com/getsentry/sentry-go"
	"os"
)

const defaultSentryDSN = "https://7e72df9c77f94e4e8e2af74c81992436@o4505437525901312.ingest.sentry.io/4505437540188160"

func init() {
	sentryDSN, hasConfig := os.LookupEnv("SENTRY_DSN")
	if !hasConfig {
		sentryDSN = defaultSentryDSN
	}
	if sentryDSN != "" {
		_ = sentry.Init(sentry.ClientOptions{Dsn: sentryDSN, TracesSampleRate: 0.6})
	}
}
