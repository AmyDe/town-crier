package uk.towncrierapp.app

import org.junit.jupiter.api.Test
import java.io.File
import javax.xml.parsers.DocumentBuilderFactory
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class BackupPolicyTest {
    private val domains = listOf("root", "file", "database", "sharedpref", "external")

    private fun parse(path: String) =
        DocumentBuilderFactory
            .newInstance()
            .apply { isNamespaceAware = true }
            .newDocumentBuilder()
            .parse(File(path))
            .documentElement

    private val application =
        parse(
            "src/main/AndroidManifest.xml",
        ).getElementsByTagName("application").item(0) as org.w3c.dom.Element

    private fun attr(name: String) = application.getAttributeNS("http://schemas.android.com/apk/res/android", name)

    @Test
    fun `the manifest turns off app backup`() {
        assertEquals("false", attr("allowBackup"))
    }

    @Test
    fun `the manifest points at both backup rule files`() {
        assertEquals("@xml/data_extraction_rules", attr("dataExtractionRules"))
        assertEquals("@xml/backup_rules", attr("fullBackupContent"))
    }

    @Test
    fun `the full backup rules exclude every domain`() {
        val excluded =
            parse(
                "src/main/res/xml/backup_rules.xml",
            ).childElements("exclude").map { it.getAttribute("domain") }
        assertEquals(domains, excluded)
    }

    @Test
    fun `the data extraction rules exclude every domain for cloud backup and device transfer`() {
        val root = parse("src/main/res/xml/data_extraction_rules.xml")
        listOf("cloud-backup", "device-transfer").forEach { section ->
            val node = root.childElements(section).single()
            assertEquals(domains, node.childElements("exclude").map { it.getAttribute("domain") })
        }
        assertTrue(root.childElements("cloud-backup").isNotEmpty())
    }

    private fun org.w3c.dom.Element.childElements(tag: String) =
        (0 until childNodes.length).map { childNodes.item(it) }.filterIsInstance<org.w3c.dom.Element>().filter {
            it.tagName ==
                tag
        }
}
