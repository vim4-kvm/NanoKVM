"""Authenticated local capture without changing the active video transport."""
import os
import requests


def fetch_snapshot(url):
    response = requests.get(
        url,
        headers={"X-NanoKVM-Internal-Token": os.environ.get("KVM_SNAPSHOT_TOKEN", "")},
        timeout=10,
        allow_redirects=False,
    )
    response.raise_for_status()
    if response.status_code != 200 or not response.headers.get("Content-Type", "").startswith("image/jpeg"):
        raise ValueError("Capture did not return a JPEG frame")
    return response.content
