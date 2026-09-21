// @ts-check
import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";

// The site is served from a project page, so every asset path is prefixed with
// the repository name. Getting `base` wrong breaks images and internal links
// without failing the build, so it is kept next to `site` where both are
// obvious.
export default defineConfig({
  site: "https://12fahed.github.io",
  base: "/cca",
  integrations: [
    starlight({
      title: "cca",
      description:
        "Analyze your local Claude Code usage: tokens, cost at API list prices, and a rough water estimate.",
      logo: {
        src: "./public/cca-logo.svg",
        alt: "cca",
        replacesTitle: false,
      },
      favicon: "/cca-logo.svg",
      social: [
        {
          icon: "github",
          label: "GitHub",
          href: "https://github.com/12fahed/cca",
        },
      ],
      editLink: {
        baseUrl: "https://github.com/12fahed/cca/edit/main/docs/",
      },
      lastUpdated: true,
      tableOfContents: { minHeadingLevel: 2, maxHeadingLevel: 3 },
      customCss: ["./src/styles/custom.css"],
      sidebar: [
        {
          label: "Start here",
          items: [
            { label: "What is cca?", slug: "start/what-is-cca" },
            { label: "Installation", slug: "start/installation" },
            { label: "Quick start", slug: "start/quick-start" },
          ],
        },
        {
          label: "Guides",
          items: [
            { label: "Reading the summary", slug: "guides/summary" },
            { label: "Breakdown views", slug: "guides/breakdowns" },
            { label: "Time ranges", slug: "guides/time-ranges" },
            { label: "JSON and CSV output", slug: "guides/machine-output" },
            { label: "Configuration", slug: "guides/configuration" },
            { label: "Troubleshooting", slug: "guides/troubleshooting" },
          ],
        },
        {
          label: "Reference",
          items: [
            { label: "Commands", slug: "reference/commands" },
            { label: "Flags", slug: "reference/flags" },
            { label: "Rate table", slug: "reference/pricing" },
            { label: "JSON schema", slug: "reference/json" },
            { label: "Exit codes", slug: "reference/exit-codes" },
          ],
        },
        {
          label: "How it works",
          items: [
            { label: "Where the numbers come from", slug: "internals/data" },
            { label: "How cost is calculated", slug: "internals/cost" },
            { label: "The water estimate", slug: "internals/water" },
            { label: "Accuracy and limits", slug: "internals/accuracy" },
            { label: "Privacy", slug: "internals/privacy" },
          ],
        },
        {
          label: "Project",
          items: [
            { label: "Contributing", slug: "project/contributing" },
            { label: "License", slug: "project/license" },
          ],
        },
      ],
    }),
  ],
});
