#!/usr/bin/env python3

# Copyright 2022 The ModelMesh Contributors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.#

# This script finds MD-style links and URLs in markdown file and verifies
# that the referenced resources exist.

import concurrent.futures
import itertools
import re

from glob import glob
from os import environ as env
from os.path import abspath, dirname, exists, relpath
from random import randint
from time import sleep
from urllib.request import Request, urlopen
from urllib.parse import urlparse
from urllib.error import URLError, HTTPError

GITHUB_REPO = env.get("GITHUB_REPO", "https://github.com/kserve/modelmesh-serving/")

md_file_path_expressions = [
    "/**/*.md",
]

excluded_paths = [
    "node_modules",
    "temp",
]

script_folder = abspath(dirname(__file__))
project_root_dir = abspath(dirname(script_folder))
github_repo_master_path = "{}/blob/master".format(GITHUB_REPO.rstrip("/"))

parallel_requests = 60  # GitHub rate limiting is 60 requests per minute, then we sleep a bit

url_status_cache = dict()


def find_md_files() -> [str]:

    # print("Checking for Markdown files here:\n")
    # for path_expr in md_file_path_expressions:
    #     print("  " + path_expr.lstrip("/"))
    # print("")

    list_of_lists = [glob(project_root_dir + path_expr, recursive=True)
                     for path_expr in md_file_path_expressions]

    flattened_list = list(itertools.chain(*list_of_lists))

    filtered_list = [path for path in flattened_list
                     if not any(s in path for s in excluded_paths)]

    return sorted(filtered_list)


def get_links_from_md_file(md_file_path: str) -> [(int, str, str)]:  # -> [(line, link_text, URL)]

    with open(md_file_path, "r") as f:
        try:
            md_file_content = f.read()
        except ValueError as e:
            print(f"Error trying to load file {md_file_path}")
            raise e

    folder = relpath(dirname(md_file_path), project_root_dir)

    # replace relative links that are siblings to the README, i.e. [link text](FEATURES.md)
    md_file_content = re.sub(
        r"\[([^]]+)\]\((?!http|#|/)([^)]+)\)",
        r"[\1]({}/{}/\2)".format(github_repo_master_path, folder).replace("/./", "/"),
        md_file_content)

    # replace links that are relative to the project root, i.e. [link text](/sdk/FEATURES.md)
    md_file_content = re.sub(
        r"\[([^]]+)\]\(/([^)]+)\)",
        r"[\1]({}/\2)".format(github_repo_master_path),
        md_file_content)

    # find all the links
    line_text_url = []
    for line_number, line_text in enumerate(md_file_content.splitlines()):

        # find markdown-styled links [text](url)
        for (link_text, url) in re.findall(r"\[([^]]+)\]\((%s[^)]+)\)" % "http", line_text):
            line_text_url.append((line_number + 1, link_text, url))

        # find plain http(s)-style links
        for url in re.findall(r"[\n\r\s\"'](https?://[^\s]+)[\n\r\s\"']", line_text):
            if not any(s in url for s in ["play.min.io", "oauth2.googleapis.com"]):
                try:
                    urlparse(url)
                    line_text_url.append((line_number + 1, "", url))
                except URLError:
                    pass

    # return completed links
    return line_text_url


def test_url(file: str, line: int, text: str, url: str) -> (str, int, str, str, int):  # (file, line, text, url, status)

    short_url = url.split("#", maxsplit=1)[0]
    status = 0

    if short_url not in url_status_cache:

        # mind GitHub rate limiting, use local files to verify link
        if short_url.startswith(github_repo_master_path):
            local_path = short_url.replace(github_repo_master_path, "")
            if exists(abspath(project_root_dir + local_path)):
                status = 200
            else:
                status = 404
        else:
            status = request_url(short_url, method="HEAD", timeout=5)

        if status == 405:  # method not allowed, use GET instead of HEAD
            status = request_url(short_url, method="GET", timeout=5)

        if status == 429:  # GitHub rate limiting, try again after 1 minute
            sleep(randint(60, 90))
            status = request_url(short_url, method="HEAD", timeout=5)

        url_status_cache[short_url] = status

    status = url_status_cache[short_url]

    return file, line, text, url, status


def request_url(short_url, method="HEAD", timeout=5) -> int:
    try:
        req = Request(short_url, method=method)
        resp = urlopen(req, timeout=timeout)
        status = resp.code
    except HTTPError as e:
        status = e.code
    except URLError as e:
        status = 555
        # print(e.reason, short_url)

    return status


def verify_urls_concurrently(file_line_text_url: [(str, int, str, str)]) -> [(str, int, str, str)]:
    file_line_text_url_status = []

    with concurrent.futures.ThreadPoolExecutor(max_workers=parallel_requests) as executor:
        check_urls = (
            executor.submit(test_url, file, line, text, url)
            for (file, line, text, url) in file_line_text_url
        )
        for url_check in concurrent.futures.as_completed(check_urls):
            try:
                file, line, text, url, status = url_check.result()
                file_line_text_url_status.append((file, line, text, url, status))
            except Exception as e:
                print(str(type(e)))
                file_line_text_url_status.append((file, line, text, url, 500))
            finally:
                print("{}/{}".format(len(file_line_text_url_status),
                                     len(file_line_text_url)), end="\r")

    return file_line_text_url_status


def verify_doc_links() -> [(str, int, str, str)]:

    # 1. find all relevant Markdown files
    md_file_paths = find_md_files()

    # 2. extract all links with text and URL
    file_line_text_url = [
        (file, line, text, url)
        for file in md_file_paths
        for (line, text, url) in get_links_from_md_file(file)
    ]

    # 3. validate the URLs
    file_line_text_url_status = verify_urls_concurrently(file_line_text_url)

    # 4. filter for the invalid URLs (status 404: "Not Found") to be reported
    file_line_text_url_404 = [(f, l, t, u, s)
                              for (f, l, t, u, s) in file_line_text_url_status
                              if s == 404]

    # 5. print some stats for confidence
    print("{} {} links ({} unique URLs) in {} Markdown files.\n".format(
        "Checked" if file_line_text_url_404 else "Verified",
        len(file_line_text_url_status),
        len(url_status_cache),
        len(md_file_paths)))

    # 6. report invalid links, exit with error for CI/CD
    if file_line_text_url_404:

        for (file, line, text, url, status) in file_line_text_url_404:
            print("{}:{}: {} -> {}".format(
                relpath(file, project_root_dir), line,
                url.replace(github_repo_master_path, ""), status))

        # print a summary line for clear error discovery at the bottom of Travis job log
        print("\nERROR: Found {} invalid Markdown links".format(
            len(file_line_text_url_404)))

        exit(1)


if __name__ == '__main__':
    verify_doc_links()
