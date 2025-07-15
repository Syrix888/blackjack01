
import { Container, getRandom } from "@cloudflare/containers";

export class Backend extends Container {
  defaultPort = 8080; // Requests are sent to port 8080 in the container
  sleepAfter = "2h";  // Only sleep a container if it hasn't gotten requests in 2 hours
}

export interface Env {
  BACKEND: DurableObjectNamespace<Backend>;
}

const INSTANCE_COUNT = 3; // Or however many you want to run

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);

    // Directly forward /game/* requests to the backend container
    if (url.pathname.startsWith("/game/") || url.pathname.startsWith("/game")) {
      // Pick a backend container instance (basic round robin/random)
      const containerInstance = await getRandom(env.BACKEND, INSTANCE_COUNT);
      return containerInstance.fetch(request);
    }

    // Optionally: serve a health check or root page
    if (url.pathname === "/") {
      return new Response("Blackjack Cloudflare Container Worker", { status: 200 });
    }

    // All other requests: 404
    return new Response("Not Found", { status: 404 });
  },
};
