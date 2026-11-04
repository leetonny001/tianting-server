# AC-OPS-06-1：删除 Grafana 运行时加的字段，再和仓库里的 JSON 比对。用法：jq -S -f normalize.jq 文件
def strip: walk(if type == "object" then del(.id, .version, .iteration, .gnetId, .pluginVersion) else . end);
(.dashboard // .) | strip
