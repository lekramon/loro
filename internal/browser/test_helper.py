import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("helper", Path(__file__).with_name("helper.py"))
helper = importlib.util.module_from_spec(spec)
spec.loader.exec_module(helper)


class ParserTests(unittest.TestCase):
    def test_mixed_rows_paging_and_unknowns(self):
        row = '<tr><td class="coll-1"><a href="/sub/{category}/">Type</a><a href="/torrent/{id}/Title/">Title &amp; More</a></td><td class="coll-2">?</td><td class="coll-4">unknown</td></tr>'
        html = '<table class="table-list">' + row.format(category=42, id=1) + row.format(category=41, id=2) + '</table><a href="/search/title/2/">Next</a>'
        result = helper.parse_page(html, "search", "https://example.test/search/title/1/")
        self.assertEqual(result["rawCount"], 2)
        self.assertEqual(len(result["rows"]), 1)
        self.assertEqual(result["rows"][0]["title"], "Title & More")
        self.assertTrue(result["hasMore"])

    def test_changed_page_empty_and_magnet_ambiguity(self):
        self.assertEqual(helper.parse_page("<html>changed</html>", "search", "https://example.test/1/")["error"], "source-invalid-response")
        self.assertEqual(helper.parse_page('<table class="table-list"></table>', "search", "https://example.test/1/")["error"], "source-invalid-response")
        self.assertEqual(helper.parse_page("No results were returned", "search", "https://example.test/1/")["rows"], [])
        magnet = "magnet:?xt=urn:btih:" + "a" * 40
        self.assertEqual(helper.parse_page(f'<a href="{magnet}">one</a><a href="{magnet}">same</a>', "detail", "https://example.test/")["magnet"], magnet)
        self.assertEqual(helper.parse_page(f'<a href="{magnet}">one</a><a href="{magnet}b">two</a>', "detail", "https://example.test/")["error"], "ambiguous-reference")

    def test_live_category_shape_and_forty_raw_positions(self):
        rows = ''.join(f'<tr><td class="coll-1"><a href="/sub/{54 if i % 2 == 0 else 41}/0/">Type</a><a href="/torrent/{i+1}/Title/">Movie</a></td></tr>' for i in range(40))
        result = helper.parse_page('<table class="table-list">' + rows + '</table>', "search", "https://example.test/category-search/title/Movies/1/")
        self.assertEqual(result["rawCount"], 40)
        self.assertEqual(len(result["rows"]), 20)
        self.assertEqual([r["position"] for r in result["rows"]], list(range(0, 40, 2)))
        self.assertTrue(result["hasMore"])


if __name__ == "__main__":
    unittest.main()
