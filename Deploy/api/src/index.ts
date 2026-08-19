import { Container, getContainer } from "@cloudflare/containers";

export class DealSignal extends Container {
	defaultPort = 8080;
	sleepAfter = "10m";

	envVars = {
		MESSAGE: "Hello from Cloudflare Containers!",
		DB_HOST: "db.example.com",
		LOG_LEVEL: "debug",
    };

}

export default {
	async fetch(
		request: Request,
		env: { DEALSIGNAL: DurableObjectNamespace },
	): Promise<Response> {
		return getContainer(env.DEALSIGNAL).fetch(request);
	},
};
