"""Extend only the pinned Paste pairing link dialog to import direct TCP URIs."""
import gzip
import hashlib
from pathlib import Path
import re

SERVICE = Path(__file__).resolve().parent
MODAL_SHA256 = "9e63aa44ce79b85ba5ed942d9627ecf61d84ccd70f6aaed4667da9bd3fa82764"

DIRECT_BRANCH = r'''if(e.startsWith("tcp://")){
  try {
    R(!0); L("");
    await infraPairDirectUri(e, {
      parse: y.parseConnectionUri, probe: infraProbeDirect, hosts: M,
      onSaved: S, onClose: V
    });
  } catch(error) {
    const message = error?.message === "Incorrect password"
      ? "Incorrect password"
      : "Unable to connect. Check the link and try again.";
    L(message);
    if(!P) c.default.alert(z("pairing.link.alert.failedTitle"), message);
  } finally { R(!1); }
  return;
}'''


def replace_once(source, old, new):
    if source.count(old) != 1:
        raise RuntimeError("Pinned Paseo pairing dialog changed; review the direct-link extension")
    return source.replace(old, new, 1)


def write_asset(path, content):
    # Build inputs are hard links to the verified snapshot: replace the inode.
    temporary = path.with_name(path.name + ".pairing.tmp")
    temporary.write_bytes(content)
    temporary.chmod(0o644)
    temporary.replace(path)


def patch_assets(web_ui):
    index = web_ui / "index.html"
    html = index.read_text()
    assets = re.findall(r'src="(/_expo/static/js/web/index-[a-f0-9]+\.js)"', html)
    if len(assets) != 1:
        raise RuntimeError("Expected one pinned Paseo web bundle")
    source_path = web_ui / assets[0].lstrip("/")
    source = source_path.read_text()
    modals = [line for line in source.splitlines() if "_e.PairLinkModal=function" in line]
    if len(modals) != 1 or hashlib.sha256(modals[0].encode()).hexdigest() != MODAL_SHA256:
        raise RuntimeError("Unsupported Paseo pairing dialog; review the direct-link extension")
    modal = replace_once(modals[0], "{upsertConnectionFromOfferUrl:w}",
                         "{upsertConnectionFromOfferUrl:w,probeAndUpsertDirectConnection:infraProbeDirect}")
    modal = replace_once(modal, 'if(!e.includes("#offer="))', DIRECT_BRANCH + 'if(!e.includes("#offer="))')
    modal = replace_once(modal, "[M,V,P,O,S,z,w]", "[M,V,P,O,S,z,w,infraProbeDirect]")
    modal = replace_once(modal, 'placeholder:"https://app.paseo.sh/#offer=..."',
                         'placeholder:"tcp://host:443?ssl=true&password=..."')
    helper = (SERVICE / "direct-pairing.js").read_text()
    modal = replace_once(modal, "},5024,[", "\n" + helper + "\n},5024,[")
    bundle = replace_once(source, modals[0], modal).encode()
    target = source_path.with_name("index-" + hashlib.sha256(bundle).hexdigest()[:32] + ".js")
    write_asset(target, bundle)
    write_asset(target.with_suffix(".js.gz"), gzip.compress(bundle, compresslevel=6, mtime=0))
    updated_html = html.replace(source_path.name, target.name).encode()
    write_asset(index, updated_html)
    if index.with_suffix(".html.gz").exists():
        write_asset(index.with_suffix(".html.gz"), gzip.compress(updated_html, mtime=0))
    source_path.unlink()
    source_path.with_suffix(".js.gz").unlink(missing_ok=True)
    return target
