#!/bin/sh
set -e

wget https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64 -O /usr/local/bin/jq
chmod +x /usr/local/bin/jq

terraform state rm heroku_addon.postgresql
terraform import heroku_addon.postgresql 791cf73f-de13-4b0c-bec1-53a290abd5db

terraform state pull > state
cat > run.jq << EOF
(.resources[] | select(.name=="postgresql") | .instances[0].attributes) += {"config": {"version": "11"}}
EOF
jq -f run.jq state > state_with_config
terraform state push -force state_with_config
