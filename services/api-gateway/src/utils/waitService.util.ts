export async function waitForService(url: string, retries = 360, delay = 5000) {
  for (let i = 0; i < retries; i++) {
    try {
      const res = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query: '{ __typename }' }),
      });
      if (res.ok) {
        const body = await res.json();
        if (body?.data?.__typename) {
          console.log(`Service at ${url} is ready.`);
          return;
        }
      }
    } catch (err) {
      // console.log(`Waiting for ${url}...`);
    }
    await new Promise((r) => setTimeout(r, delay));
  }
  console.warn(
    `Service at ${url} not available after ${retries} attempts, starting anyway...`,
  );
}