export function LoadingSpinner() {
  return (
    <div className="flex items-center justify-center p-10">
      <div className="flex items-center gap-3 rounded-md border border-[#606060] bg-[#464646] px-4 py-3 text-[#b0b0b0]">
        <div className="h-4 w-4 animate-spin rounded-full border-2 border-[#007acc] border-t-transparent" />
        <span className="text-sm">Loading…</span>
      </div>
    </div>
  );
}

export function LoadingCard() {
  return (
    <div className="animate-pulse rounded-md border border-[#606060] bg-[#464646] p-5">
      <div className="mb-3 h-3 w-1/4 rounded bg-[#606060]"></div>
      <div className="mb-3 h-6 w-1/2 rounded bg-[#606060]"></div>
      <div className="h-3 w-3/4 rounded bg-[#606060]"></div>
    </div>
  );
}
