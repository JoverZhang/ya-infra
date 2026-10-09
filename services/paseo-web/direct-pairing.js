// Called from the pinned UI's PairLinkModal. Reuse its normal direct connection
// parser and probe/save operation, including password admission and server ID.
async function infraPairDirectUri(uri, {parse, probe, hosts, onSaved, onClose}) {
  const connection = parse(uri);
  const endpoint = connection.isIpv6
    ? `[${connection.host}]:${connection.port}`
    : `${connection.host}:${connection.port}`;
  const result = await probe({
    endpoint,
    useTls: connection.useTls,
    ...(connection.password ? {password: connection.password} : {})
  });
  onSaved?.({
    ...result,
    isNewHost: !hosts.some(host => host.serverId === result.serverId)
  });
  onClose();
}
