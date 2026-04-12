import path from "node:path";
import type { NextConfig } from "next";
import { withSentryConfig } from "@sentry/nextjs";
import createNextIntlPlugin from "next-intl/plugin";

const hasSentry = !!process.env.NEXT_PUBLIC_SENTRY_DSN;

const nextConfig: NextConfig = {
	output: "standalone",
	outputFileTracingRoot: path.resolve(process.cwd(), "../.."),
	async headers() {
		return [
			{
				source: "/(.*)",
				headers: [
					{
						key: "X-Content-Type-Options",
						value: "nosniff",
					},
					{
						key: "X-Frame-Options",
						value: "DENY",
					},
					{
						key: "Referrer-Policy",
						value: "strict-origin-when-cross-origin",
					},
				],
			},
		];
	},
};

const withNextIntl = createNextIntlPlugin();

export default hasSentry
	? withSentryConfig(withNextIntl(nextConfig), {
			org: process.env.SENTRY_ORG,
			project: process.env.SENTRY_PROJECT,
			silent: !process.env.CI,
		})
	: withNextIntl(nextConfig);
