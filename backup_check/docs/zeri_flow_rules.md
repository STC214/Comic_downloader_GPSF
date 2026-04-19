# Zeri Flow Rules

This document records the current `zeri` page-processing rules so the browser flow can be rebuilt cleanly.

## Inputs

- User entry URL is the summary/article page.
- The workflow must resolve the real reader page before collecting any downloadable images.

## Summary page rules

1. The summary page contains the manga title.
2. The summary page contains the expected page count in text like:
   - `Length : 14 pages`
3. The number in `Length : N pages` is the expected image count for the full reader flow.
4. The summary page contains exactly two `div.row` blocks that point to the reader page.
5. The reader page URL must be resolved from those `div.row` blocks first.
6. The summary page must not be used as the final download source.
7. The summary page must not download cover art as chapter content.

## Reader page rules

1. The reader page must be entered before collecting any final images.
2. After the reader page is open, click the `100%` button once.
3. Only after the `100%` click should the page be re-read for image URLs.
4. Each reader pagination page must also follow the same order:
   - open the page
   - click `100%`
   - re-collect the page images
5. The reader pager exists in both:
   - `#page_num1`
   - `#page_num2`
6. Pagination links from both pager blocks must be merged and deduplicated.

## Image selection rules

1. Reader images must be taken only from the reader page content, not the summary/list page.
2. A target image URL must contain two shared numeric signatures.
3. Each shared signature must be at least 6 digits long.
4. All valid reader image URLs must share the same pair of numeric signatures.
5. If cover images or list thumbnails do not match those shared signatures, they must be excluded.
6. After filtering, the final collected image count should match the expected page count from the summary page.

## Navigation rules

1. The browser should start from the user-provided summary page URL.
2. Resolve the reader page URL from the summary page DOM.
3. Enter the reader page in the same browser session when possible.
4. Do not open a second browser path for a different mode during the same task.
5. Do not leave the task stuck on the summary page when the reader page is available.

## Failure rules

- If the reader page cannot be resolved, fail early and log the summary page HTML snapshot.
- If the reader page opens but the image count is lower than the summary page `Length : N pages`, treat it as a mismatch.
- If the final collected images are empty, do not fall back to list-page cover images.

## Notes for tomorrow

- The current refactor should be split into:
  - summary page parsing
  - reader page activation
  - reader page image filtering
  - pagination handling
  - download execution
- Keep the summary page and reader page responsibilities separate.
- Prefer explicit logging of:
  - resolved reader URL
  - expected page count
  - pagination URLs
  - collected image count
