import Link from 'next/link';

export default function HomePage() {
  return (
    <div className="mx-auto flex w-full max-w-5xl flex-1 flex-col justify-center px-6 py-24">
      <div className="max-w-3xl">
        <p className="mb-4 text-sm font-medium text-fd-muted-foreground">Open-source metering</p>
        <h1 className="mb-6 text-4xl font-semibold tracking-tight md:text-6xl">
          Open Spanner documentation
        </h1>
        <p className="mb-8 text-lg leading-8 text-fd-muted-foreground">
          Learn how to define meters, record usage events, query buckets, and use generated SDKs for usage-based products.
        </p>
        <div className="flex flex-wrap gap-3">
          <Link
            href="/docs"
            className="inline-flex h-10 items-center rounded-md bg-fd-primary px-4 text-sm font-medium text-fd-primary-foreground"
          >
            Read the docs
          </Link>
          <Link
            href="https://github.com/ssubedir/open-spanner"
            className="inline-flex h-10 items-center rounded-md border px-4 text-sm font-medium"
          >
            GitHub
          </Link>
        </div>
      </div>
    </div>
  );
}
