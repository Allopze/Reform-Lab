import path from "node:path";
import type { NextConfig } from "next";

const isDev = process.env.NODE_ENV === "development";
const connectSrc = ["'self'"];

if (process.env.NEXT_PUBLIC_API_URL) {
	try {
		connectSrc.push(new URL(process.env.NEXT_PUBLIC_API_URL).origin);
	} catch {
		// Ignore invalid user-provided URLs and keep the default safe policy.
	}
}

if (isDev) {
	connectSrc.push("http:", "ws:");
}

const cspHeader = `
	default-src 'self';
	script-src 'self' 'unsafe-inline'${isDev ? " 'unsafe-eval'" : ""};
	style-src 'self' 'unsafe-inline';
	img-src 'self' blob: data:;
	font-src 'self';
	connect-src ${connectSrc.join(" ")};
	object-src 'none';
	base-uri 'self';
	form-action 'self';
	frame-ancestors 'none';
	${isDev ? "" : "upgrade-insecure-requests;"}
`;

const nextConfig: NextConfig = {
	outputFileTracingRoot: path.resolve(process.cwd(), "../.."),
	async headers() {
		return [
			{
				source: "/(.*)",
				headers: [
					{
						key: "Content-Security-Policy",
						value: cspHeader.replace(/\n/g, " ").replace(/\s{2,}/g, " ").trim(),
					},
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

export default nextConfig;
