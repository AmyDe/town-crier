import collections
import io
import os
import sys
import unittest
import zipfile

sys.path.insert(0, os.path.dirname(__file__))
import log_candidates as lc  # noqa: E402

# Fake secrets are assembled at runtime so no literal in this file matches a provider pattern.
J = "".join
TS = "2026-10-14T10:53:20.3065693Z "
ACS = J(["endpoint=https://acs-x.uk.communication.azure.com/;", "accesskey=", "Qm9ndXNLZXlGb3JUZXN0c09ubHkxMjM0NTY3ODkwYWJjZGVmZ2hpams="])


def rules(line):
    return {rule for _, found in lc.scan_log_line(TS + line)[1] for rule in found}


class Recall(unittest.TestCase):
    def test_pulumi_output_with_acs_key(self):
        self.assertTrue(rules(f'    acsConnectionString : "{ACS}"'))

    def test_pulumi_diff_old_value_still_reported(self):
        self.assertTrue(rules(f'  ~ acsConnectionString : "{ACS}" => [secret]'))

    def test_provider_shapes_survive_noise_context(self):
        token = J(["gh", "p_", "aB3dE5fG7hI9jK1lM3nO5pQ7rS9tU1vW3xY5"])
        self.assertIn("github-token", rules(f"Cache saved with the key: {token}"))

    def test_json_secret_field(self):
        self.assertIn("json-secret-field", rules('  "client_secret": "s0me-Val1d-Secr3t-xyz"'))

    def test_storage_connection_string(self):
        line = J(["DefaultEndpointsProtocol=https;AccountName=x;", "AccountKey=", "Zm9vYmFyYmF6cXV4MTIzNDU2Nzg5MGFi"])
        self.assertTrue(rules(line))

    def test_unnamed_random_token(self):
        self.assertIn("high-entropy-token", rules("token is Xk9vT2qLm8RzP4wN7bY1cH6dJ3fG5sA0end"))


class Noise(unittest.TestCase):
    def test_ignores_structured_noise(self):
        for line in [
            "Cache saved with the key: node-cache-Linux-x64-npm-a1b2C3d4E5f6G7h8I9j0K1l2M3n4O5p6Q7r8",
            "##[command]/usr/bin/docker network create --label d2e9db github_network_f79b0f6b80e84c19bc52a1aa0a12bf72",
            '    "latestRevisionName": "ca-town-crier-api-go-dev--r36229247157-1",',
            '  "content_md5": "wGByZx9Q2kLm8RzP4wN7bQ==",',
            "256 SHA256:ePtSx9Q2kLm8RzP4wN7bY1cH6dJ3fG5sA0aB3dE5fGjI deploy key (ED25519)",
            '                                 secretRef: "auth0-client-secret"',
            '              "secretRef": "acs-connection-string",',
            '      "contentMd5": "b7NeZx9Q2kLm8RzP4wN7bQ==",',
            "Print service container logs: 5a5e9c1b2d3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e",
            "Stop and remove container: 5a5e9c1b2d3f4a5b6c7d8e9f0a1b2c3d_postgisPostgis16x34Gq9zLm",
            "  vite-auth0-client-id: rgP7x9Q2kLm8RzP4wN7bY1cH6dJ3fGjV",
            '  value: "InstrumentationKey=00000000-0000-0000-0000-000000000000;IngestionEndpoint=https://uksouth-1.in.applicationinsights.azure.com/"',
            '    devSeedReaderIdentityClientId   : "5f27a1b2-0641-4937-9268-a91bfd8191ea"',
            "echo \"::add-mask::$TOKEN\"",
            "Masked value: ***",
        ]:
            with self.subTest(line=line):
                self.assertEqual(rules(line), set())


class Aggregation(unittest.TestCase):
    def test_dedupes_by_value_and_masks_context(self):
        buf = io.BytesIO()
        with zipfile.ZipFile(buf, "w") as z:
            z.writestr("0_build.txt", f'{TS}    acsConnectionString : "{ACS}"\n')
            z.writestr("1_deploy.txt", f'{TS}    acsConnectionString : "{ACS}"\n')
        agg = collections.defaultdict(lambda: {"rules": set(), "occurrences": 0, "runs": set(), "steps": set(), "examples": []})
        lc.scan_zip(buf.getvalue(), {"workflow": "CD Dev", "run_id": 1, "attempt": 1, "url": ""}, agg)
        acs = [e for value, e in agg.items() if "accesskey=" in value]
        self.assertEqual(len(acs), 1)
        self.assertEqual(acs[0]["occurrences"], 2)
        for example in acs[0]["examples"]:
            self.assertNotIn(ACS, example["context"])

    def test_one_value_is_one_candidate_with_all_rules(self):
        hits = lc.scan_log_line(f'{TS}    acsConnectionString : "{ACS}"')[1]
        self.assertEqual(len(hits), 1)
        self.assertGreaterEqual(len(hits[0][1]), 2)


if __name__ == "__main__":
    unittest.main()
