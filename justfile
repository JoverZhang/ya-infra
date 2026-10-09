set shell := ["bash", "-euo", "pipefail", "-c"]

default:
    @just --list

build service:
    @python3 services/{{service}}/manage build

deploy service instance="":
    @python3 services/{{service}}/manage deploy {{instance}}

status service:
    @python3 services/{{service}}/manage status

logs service instance="":
    @python3 services/{{service}}/manage logs {{instance}}

login service instance:
    @python3 services/{{service}}/manage login {{instance}}

auth-status service instance:
    @python3 services/{{service}}/manage auth-status {{instance}}

stop service instance="":
    @python3 services/{{service}}/manage stop {{instance}}

restart service instance="":
    @python3 services/{{service}}/manage restart {{instance}}

verify service:
    @python3 services/{{service}}/manage verify

smoke service instance:
    @python3 services/{{service}}/manage smoke {{instance}}

adopt service legacy:
    @python3 services/{{service}}/manage adopt {{legacy}}

verify-public service instance:
    @python3 checks/{{service}}-public {{instance}}

gateway command route="":
    @python3 gateway/manage {{command}} {{route}}
