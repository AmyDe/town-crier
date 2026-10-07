import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(__file__))
import candidates  # noqa: E402

# Fake secrets are assembled at runtime so no literal in this file matches a provider pattern.
J = "".join


def rules(line, path="a/conf.go"):
    return {rule for _, _, rule, _ in candidates.scan_line(path, 1, line)}


class Recall(unittest.TestCase):
    def test_provider_shapes(self):
        cases = {
            "aws-access-key": J(["AKIA", "IOSFODNN7", "ABCDEFG"]),
            "github-token": J(["gh", "p_", "aB3dE5fG7hI9jK1lM3nO5pQ7rS9tU1vW3xY5"]),
            "stripe-key": J(["sk", "_live_", "51HxYzAbCdEfGhIjKlMn"]),
            "anthropic-key": J(["sk-", "ant-", "api03-AbCdEfGhIjKlMnOpQrStUvWx"]),
            "google-api-key": J(["AI", "za", "SyA1b2C3d4E5f6G7h8I9j0K1l2M3n4O5p6Q"]),
            "private-key-block": J(["-----BEGIN EC ", "PRIVATE KEY-----"]),
            "azure-storage-key": J(["AccountKey=", "Zm9vYmFyYmF6cXV4MTIzNDU2Nzg5MGFi"]),
            "azure-conn-secret": J(["Server=tcp:x;", "Password=", "Hunt3r2!xyz;"]),
            "azure-client-secret": J(["abc", "8Q~", "AbCdEfGhIjKlMnOpQrStUvWxYz0123456"]),
            "url-credentials": J(["postgres://admin:", "S3cr3tP4ss", "@prod.example.com/db"]),
            "jwt": J(["eyJ", "hbGciOiJIUzI1NiJ9", ".eyJ", "zdWIiOiIxMjM0NTY3ODkwIn0", ".abcdEFGHijkl"]),
        }
        for rule, value in cases.items():
            with self.subTest(rule=rule):
                self.assertIn(rule, rules(f'x := "{value}"'))

    def test_secret_named_assignments(self):
        self.assertIn("secret-assignment", rules('const apiKey = "q8Zr4LmP0vXk"'))
        self.assertIn("secret-assignment", rules("DB_PASSWORD=hunter2hunter2", ".env"))
        self.assertIn("secret-assignment", rules("  client_secret: s0me-Val1d-Secr3t", "x.yaml"))

    def test_unnamed_random_literal(self):
        self.assertIn("high-entropy-string", rules('v := "Xk9vT2qLm8RzP4wN7bY1cH6dJ3fG5sA0"'))

    def test_sensitive_file_names(self):
        for path in [".env", "certs/AuthKey_ABC.p8", "ios/GoogleService-Info.plist", "x/id_rsa"]:
            with self.subTest(path=path):
                self.assertTrue(candidates.SENSITIVE_FILE.search(path))
        self.assertFalse(candidates.SENSITIVE_FILE.search("main.go"))


class Noise(unittest.TestCase):
    def test_ignores_common_non_secrets(self):
        for line in [
            '"github.com/pulumi/pulumi-azure-native-sdk/dbforpostgresql/v3"',
            "uses: actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6",
            'id := "72fafb9e-0641-4937-9268-a91bfd8191a3"',
            "func TestApplications_RejectsUndecodableCursorWith400(t *testing.T) {",
            '  secure: AAABAJGCaZXEDG8x2DVfI2ubWIyFsVbs/Wu9vhDOqbmw9oAEqEe0Julgqp',
            'password = "changeme"',
        ]:
            with self.subTest(line=line):
                self.assertEqual(rules(line), set())

    def test_fingerprint_ignores_line_number(self):
        a = candidates.fingerprint("a.go", "jwt", "v")
        self.assertEqual(a, candidates.fingerprint("a.go", "jwt", "v"))
        self.assertNotEqual(a, candidates.fingerprint("b.go", "jwt", "v"))


if __name__ == "__main__":
    unittest.main()
