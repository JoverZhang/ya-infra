"""Report independent Cloudflare read checks without changing remote resources."""


def check_credentials(client, routes, emit=print):
    failures = []

    def check(label, operation, describe=lambda value: "read access confirmed"):
        try:
            value = operation()
            detail = describe(value)
        except (RuntimeError, ValueError) as error:
            failures.append(label)
            emit(f"FAIL {label}: {error}")
            return None
        emit(f"PASS {label}: {detail}")
        return value

    def token_status(result):
        if result.get("status") != "active":
            raise RuntimeError("saved token is not active; check expiry, activation time or disabled status")
        return "active"

    check("Token validity", client.verify_token, token_status)
    zone = check("Zone / Zone / Read", client.zone_id, lambda value: "configured zone and account matched")
    # A Zone failure must not conceal an independent Tunnel permission failure.
    check("Account / cloudflared / Read", client.configuration)
    if zone:
        for route in routes:
            check(f"Zone / DNS / Read ({route.name})", lambda route=route: client.records(route.hostname))
    else:
        emit("SKIP Zone / DNS / Read: Zone ID unavailable; fix Zone / Zone / Read or its resource scope first")
    emit("NOT TESTED write permissions: Account / cloudflared / Edit and Zone / DNS / Edit; apply exercises writes")
    if failures:
        raise RuntimeError("Cloudflare credential checks failed: " + ", ".join(failures)
                           + ". Check the existing token's permission rows, resource scopes and IP filter")
