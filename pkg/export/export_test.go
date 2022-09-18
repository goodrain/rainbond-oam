// RAINBOND, Application Management Platform
// Copyright (C) 2020-2020 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package export

import (
	"context"
	"encoding/json"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"testing"

	"github.com/goodrain/rainbond-oam/pkg/ram/v1alpha1"
	"github.com/sirupsen/logrus"
)

var kongRAM = `
{"apps": [{"cmd": "", "image": "pantsel/konga:latest", "memory": 512, "probes": [{"ID": 3443, "cmd": "", "mode": "readiness", "path": "", "port": 1337, "scheme": "tcp", "is_used": true, "probe_id": "3c1d549ab36d459dbf19b06f2c7c2e6e", "service_id": "405158ca2136824ffd7ad1df21529926", "http_header": "", "period_second": 3, "timeout_second": 30, "failure_threshold": 3, "success_threshold": 1, "initial_delay_second": 2}], "version": "latest", "category": "app_publish", "language": "", "tenant_id": "0ca9a6c8e3e74f6283cb980110fec224", "service_id": "405158ca2136824ffd7ad1df21529926", "share_type": "image", "service_key": "dae73ebd26e84a5a90a3ffd7397b6ed8", "share_image": "image.goodrain.com/655c233e59714d9191c0b9e856d84b44/405158ca2136824ffd7ad1df21529926:20191231151703", "service_name": "", "service_type": "application", "extend_method": "stateless", "port_map_list": [{"protocol": "http", "tenant_id": "0ca9a6c8e3e74f6283cb980110fec224", "port_alias": "GR5299261337", "container_port": 1337, "is_inner_service": false, "is_outer_service": true}], "service_alias": "gr529926", "service_cname": "konga", "service_image": {"hub_url": "image.goodrain.com", "hub_user": "b9775973e1d84aed9a1548e1701b05b1", "is_trust": false, "namespace": "655c233e59714d9191c0b9e856d84b44", "hub_password": "4a2a8590c197452d98d1131d2a00207d"}, "deploy_version": "20191231151703", "service_region": "rainbond", "service_source": "docker_run", "extend_method_map": {"max_node": 20, "min_node": 1, "step_node": 1, "is_restart": 0, "max_memory": 65536, "min_memory": 512, "step_memory": 128}, "mnt_relation_list": [], "service_share_uuid": "dae73ebd26e84a5a90a3ffd7397b6ed8+405158ca2136824ffd7ad1df21529926", "dep_service_map_list": [{"dep_service_key": "d5639524319a4711b7bd275d65c14469+e6dbf9f3bdd66cdb3aa5db5a2db8b4ba"}], "service_env_map_list": [{"name": "NODE_VERSION", "attr_name": "NODE_VERSION", "is_change": true, "attr_value": "10.16.3"}, {"name": "YARN_VERSION", "attr_name": "YARN_VERSION", "is_change": true, "attr_value": "1.17.3"}], "service_volume_map_list": [{"access_mode": "", "volume_name": "GR529926_1", "volume_path": "/app/kongadata", "volume_type": "share-file", "file_content": "", "volume_capacity": 0}], "service_connect_info_map_list": []}, {"cmd": "", "image": "goodrain.me/runner:latest", "memory": 512, "probes": [{"ID": 3448, "cmd": "", "mode": "readiness", "path": "", "port": 5432, "scheme": "tcp", "is_used": true, "probe_id": "8af4641ddd444dbdb479e7c61a1038dc", "service_id": "69a3fd4ecbf784fe79277150780d1113", "http_header": "", "period_second": 3, "timeout_second": 30, "failure_threshold": 3, "success_threshold": 1, "initial_delay_second": 2}], "version": "latest", "category": "application", "language": "dockerfile", "tenant_id": "0ca9a6c8e3e74f6283cb980110fec224", "service_id": "69a3fd4ecbf784fe79277150780d1113", "share_type": "image", "service_key": "c48cf52ac19a442a979d166284385e2e", "share_image": "image.goodrain.com/655c233e59714d9191c0b9e856d84b44/69a3fd4ecbf784fe79277150780d1113:20191231152640", "service_name": "", "service_type": "application", "extend_method": "state", "port_map_list": [{"protocol": "tcp", "tenant_id": "0ca9a6c8e3e74f6283cb980110fec224", "port_alias": "GR0D11135432", "container_port": 5432, "is_inner_service": true, "is_outer_service": false}], "service_alias": "gr0d1113", "service_cname": "kong-postgres", "service_image": {"hub_url": "image.goodrain.com", "hub_user": "70a4f58f8acb47eca8ca18344d3cb8f7", "is_trust": false, "namespace": "655c233e59714d9191c0b9e856d84b44", "hub_password": "d2c0fa79510642288fc1d03af281297c"}, "deploy_version": "20191231152640", "service_region": "rainbond", "service_source": "source_code", "extend_method_map": {"max_node": 1, "min_node": 1, "step_node": 1, "is_restart": 0, "max_memory": 65536, "min_memory": 512, "step_memory": 128}, "mnt_relation_list": [], "service_share_uuid": "c48cf52ac19a442a979d166284385e2e+69a3fd4ecbf784fe79277150780d1113", "dep_service_map_list": [], "service_env_map_list": [{"name": "TZ", "attr_name": "TZ", "is_change": true, "attr_value": "Aisa/Shanghai"}, {"name": "LANG", "attr_name": "LANG", "is_change": true, "attr_value": "en_US.utf8"}, {"name": "PGDATA", "attr_name": "PGDATA", "is_change": true, "attr_value": "/var/lib/postgresql/data"}, {"name": "PG_MAJOR", "attr_name": "PG_MAJOR", "is_change": true, "attr_value": "10"}, {"name": "PG_VERSION", "attr_name": "PG_VERSION", "is_change": true, "attr_value": "10.11"}, {"name": "", "attr_name": "POSTGRES_USER", "is_change": true, "attr_value": "kong"}, {"name": "", "attr_name": "POSTGRES_DB", "is_change": true, "attr_value": "kong"}], "service_volume_map_list": [{"access_mode": "", "volume_name": "GR0D1113_1", "volume_path": "/var/lib/postgresql/data", "volume_type": "share-file", "file_content": "", "volume_capacity": 0}], "service_connect_info_map_list": []}, {"cmd": "", "image": "kong:latest", "memory": 4096, "probes": [{"ID": 3438, "cmd": "", "mode": "readiness", "path": "", "port": 8001, "scheme": "tcp", "is_used": true, "probe_id": "d99a1d9c5b824b12a293da382199c3b6", "service_id": "e6dbf9f3bdd66cdb3aa5db5a2db8b4ba", "http_header": "", "period_second": 3, "timeout_second": 30, "failure_threshold": 3, "success_threshold": 1, "initial_delay_second": 2}], "version": "latest", "category": "app_publish", "language": "", "tenant_id": "0ca9a6c8e3e74f6283cb980110fec224", "service_id": "e6dbf9f3bdd66cdb3aa5db5a2db8b4ba", "share_type": "image", "service_key": "d5639524319a4711b7bd275d65c14469", "share_image": "image.goodrain.com/655c233e59714d9191c0b9e856d84b44/e6dbf9f3bdd66cdb3aa5db5a2db8b4ba:20191231141811", "service_name": "", "service_type": "application", "extend_method": "stateless", "port_map_list": [{"protocol": "http", "tenant_id": "0ca9a6c8e3e74f6283cb980110fec224", "port_alias": "GRB8B4BA8000", "container_port": 8000, "is_inner_service": true, "is_outer_service": false}, {"protocol": "http", "tenant_id": "0ca9a6c8e3e74f6283cb980110fec224", "port_alias": "GRB8B4BA8001", "container_port": 8001, "is_inner_service": true, "is_outer_service": true}, {"protocol": "http", "tenant_id": "0ca9a6c8e3e74f6283cb980110fec224", "port_alias": "GRB8B4BA8443", "container_port": 8443, "is_inner_service": true, "is_outer_service": false}, {"protocol": "http", "tenant_id": "0ca9a6c8e3e74f6283cb980110fec224", "port_alias": "GRB8B4BA8444", "container_port": 8444, "is_inner_service": true, "is_outer_service": false}], "service_alias": "grb8b4ba", "service_cname": "kong", "service_image": {"hub_url": "image.goodrain.com", "hub_user": "1fc68efb3edf44ab996701bb04d0119f", "is_trust": false, "namespace": "655c233e59714d9191c0b9e856d84b44", "hub_password": "21087aad3f04467085154cc14025cf89"}, "deploy_version": "20191231141811", "service_region": "rainbond", "service_source": "docker_run", "extend_method_map": {"max_node": 20, "min_node": 1, "step_node": 1, "is_restart": 0, "max_memory": 65536, "min_memory": 4096, "step_memory": 128}, "mnt_relation_list": [], "service_share_uuid": "d5639524319a4711b7bd275d65c14469+e6dbf9f3bdd66cdb3aa5db5a2db8b4ba", "dep_service_map_list": [{"dep_service_key": "c48cf52ac19a442a979d166284385e2e+69a3fd4ecbf784fe79277150780d1113"}], "service_env_map_list": [{"name": "KONG_VERSION", "attr_name": "KONG_VERSION", "is_change": true, "attr_value": "1.4.2"}, {"name": "KONG_SHA256", "attr_name": "KONG_SHA256", "is_change": true, "attr_value": "edf917d956d697abb70f5f3f630d420ee699c6428bf953221cd8548eda490dcf"}, {"name": "", "attr_name": "KONG_DATABASE", "is_change": true, "attr_value": "postgres"}, {"name": "", "attr_name": "KONG_PG_HOST", "is_change": true, "attr_value": "127.0.0.1"}, {"name": "", "attr_name": "KONG_CASSANDRA_CONTACT_POINTS", "is_change": true, "attr_value": "kong-database"}, {"name": "", "attr_name": "KONG_PROXY_ACCESS_LOG", "is_change": true, "attr_value": "/dev/stdout"}, {"name": "", "attr_name": "KONG_ADMIN_ACCESS_LOG", "is_change": true, "attr_value": "/dev/stdout"}, {"name": "", "attr_name": "KONG_PROXY_ERROR_LOG", "is_change": true, "attr_value": "/dev/stderr"}, {"name": "", "attr_name": "KONG_ADMIN_ERROR_LOG", "is_change": true, "attr_value": "/dev/stderr"}, {"name": "", "attr_name": "KONG_ADMIN_LISTEN", "is_change": true, "attr_value": "0.0.0.0:8001, 0.0.0.0:8444 ssl"}, {"name": "", "attr_name": "KONG_NGINX_WORKER_PROCESSES", "is_change": true, "attr_value": "5"}], "service_volume_map_list": [], "service_connect_info_map_list": [], "service_related_plugin_config": [{"ID": 1029, "attr": [{"ID": 3898, "attrs": "{\"OPEN\": \"YES\"}", "protocol": "http", "injection": "auto", "plugin_id": "2f929983fc1349ad8569b7750122bc5a", "service_id": "e6dbf9f3bdd66cdb3aa5db5a2db8b4ba", "create_time": "2019-12-31 14:31:56", "build_version": "20190617190729", "container_port": 8000, "dest_service_id": "", "service_meta_type": "upstream_port", "dest_service_alias": ""}, {"ID": 3899, "attrs": "{\"OPEN\": \"YES\"}", "protocol": "http", "injection": "auto", "plugin_id": "2f929983fc1349ad8569b7750122bc5a", "service_id": "e6dbf9f3bdd66cdb3aa5db5a2db8b4ba", "create_time": "2019-12-31 14:31:56", "build_version": "20190617190729", "container_port": 8001, "dest_service_id": "", "service_meta_type": "upstream_port", "dest_service_alias": ""}, {"ID": 3900, "attrs": "{\"OPEN\": \"YES\"}", "protocol": "http", "injection": "auto", "plugin_id": "2f929983fc1349ad8569b7750122bc5a", "service_id": "e6dbf9f3bdd66cdb3aa5db5a2db8b4ba", "create_time": "2019-12-31 14:31:56", "build_version": "20190617190729", "container_port": 8443, "dest_service_id": "", "service_meta_type": "upstream_port", "dest_service_alias": ""}, {"ID": 3901, "attrs": "{\"OPEN\": \"YES\"}", "protocol": "http", "injection": "auto", "plugin_id": "2f929983fc1349ad8569b7750122bc5a", "service_id": "e6dbf9f3bdd66cdb3aa5db5a2db8b4ba", "create_time": "2019-12-31 14:31:56", "build_version": "20190617190729", "container_port": 8444, "dest_service_id": "", "service_meta_type": "upstream_port", "dest_service_alias": ""}], "plugin_id": "2f929983fc1349ad8569b7750122bc5a", "plugin_key": "perf_analyze_plugin", "service_id": "e6dbf9f3bdd66cdb3aa5db5a2db8b4ba", "create_time": "2019-12-31 14:31:56", "build_version": "20190617190729", "plugin_status": true, "service_meta_type": ""}]}], "plugins": [{"ID": 235, "desc": "实时分析应用的吞吐率、响应时间、在线人数等指标", "image": "goodrain.me/tcm", "origin": "local_market", "category": "analyst-plugin:perf", "code_repo": "", "plugin_id": "2f929983fc1349ad8569b7750122bc5a", "plugin_key": "perf_analyze_plugin", "create_time": "2019-06-17 19:07:29", "plugin_name": "gr2f9299", "share_image": "image.goodrain.com/655c233e59714d9191c0b9e856d84b44/plugin_tcm_2f929983fc1349ad8569b7750122bc5a:latest_201961719729234617530_20190617190729", "build_source": "image", "plugin_alias": "服务实时性能分析", "plugin_image": {"hub_url": "image.goodrain.com", "hub_user": "bb4bb125646f4facb6362f69be3f2b61", "is_trust": false, "namespace": "655c233e59714d9191c0b9e856d84b44", "hub_password": "c453da8f5fb2422db8c80337556940a3"}, "build_version": "20190617190729", "config_groups": [{"ID": 229, "options": [{"ID": 1771, "protocol": "http,mysql", "attr_info": "是否开启当前端口分析，用户自助选择服务端口", "attr_name": "OPEN", "attr_type": "radio", "is_change": true, "plugin_id": "2f929983fc1349ad8569b7750122bc5a", "build_version": "20190617190729", "attr_alt_value": "YES,NO", "service_meta_type": "upstream_port", "attr_default_value": "YES"}], "injection": "auto", "plugin_id": "2f929983fc1349ad8569b7750122bc5a", "config_name": "端口是否开启分析", "build_version": "20190617190729", "service_meta_type": "upstream_port"}], "origin_share_id": "perf_analyze_plugin"}], "group_key": "7f1456581bb043aab545375b228a8635", "group_name": "Kong", "group_version": "v1.4.2", "template_version": "v2"}`

func TestExportDockerCompose(t *testing.T) {
	var ram v1alpha1.RainbondApplicationConfig
	json.Unmarshal([]byte(kongRAM), &ram)
	//c, _ := client.NewEnvClient()
	containerdClient, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		t.Fatal(err)
	}
	cctx := namespaces.WithNamespace(context.Background(), "rainbond")
	imageService := containerdClient.ImageService()
	c := ContainerdAPI{
		ImageService:     imageService,
		CCtx:             cctx,
		ContainerdClient: containerdClient,
	}
	exp := New(DC, "/tmp/dc", ram, c, logrus.StandardLogger())
	re, err := exp.Export()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(re)
}

func TestExportRAM(t *testing.T) {
	var ram v1alpha1.RainbondApplicationConfig
	json.Unmarshal([]byte(kongRAM), &ram)
	//c, _ := client.NewEnvClient()
	containerdClient, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		t.Fatal(err)
	}
	cctx := namespaces.WithNamespace(context.Background(), "rainbond")
	imageService := containerdClient.ImageService()
	c := ContainerdAPI{
		ImageService:     imageService,
		CCtx:             cctx,
		ContainerdClient: containerdClient,
	}
	exp := New(RAM, "/tmp/dc", ram, c, logrus.StandardLogger())
	re, err := exp.Export()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(re)
}
