import { defineConfig, triggers } from "cf/config";

// goFinder 官网：静态资源在 public/，部署用 `cf deploy`（见 docs/website.md）
export default defineConfig({
	worker: {
		name: "gofinder",
		compatibilityDate: "2026-10-19",
		triggers: [
			triggers.fetch({
				pattern: "gofinder.ryou.im/*",
				zone: "ryou.im",
			}),
		],
	},
});
