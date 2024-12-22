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

package plugin

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	processCallTimeUsage = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "plugin_process_call_time_usage_seconds",
			Help:    "The time usage of process plugin call.",
			Buckets: prometheus.ExponentialBuckets(0.1, 5, 5),
		},
		[]string{"plugin_name"},
	)
)

func init() {
	prometheus.MustRegister(
		processCallTimeUsage,
	)
}
