import { cn } from "@/lib/utils";
import type { FeedEvent, PriceImpulseAlert } from "@/types";
import { Copy } from "lucide-react";
import { useEffect, useState } from "react";
import { CandleChart } from "./CandleChart";

interface Props {
	event: FeedEvent;
	chartsOpen: boolean;
}

export function FeedRow({ event, chartsOpen }: Props) {
	const [isEntering, setIsEntering] = useState(true);
	const [enterActive, setEnterActive] = useState(false);

	useEffect(() => {
		const frame = requestAnimationFrame(() => setEnterActive(true));
		const timer = window.setTimeout(() => setIsEntering(false), 360);

		return () => {
			cancelAnimationFrame(frame);
			window.clearTimeout(timer);
		};
	}, []);

	return (
		<div
			className={cn(
				"relative border-b border-border/40 last:border-0",
				isEntering && "feed-row-enter",
				isEntering && enterActive && "feed-row-enter-active",
			)}
		>
			<div key={event.updatedAt} className="feed-flash-bar" />
			{event.type === "PRICE_IMPULSE" ? (
				<PriceImpulseRow
					event={event as FeedEvent<PriceImpulseAlert>}
					chartsOpen={chartsOpen}
				/>
			) : (
				<GenericRow event={event} />
			)}
		</div>
	);
}

function PriceImpulseRow({
	event,
	chartsOpen,
}: {
	event: FeedEvent<PriceImpulseAlert>;
	chartsOpen: boolean;
}) {
	const a = event.data;
	const [chartOpen, setChartOpen] = useState(chartsOpen);
	const [copied, setCopied] = useState(false);

	const isUp = a.direction === "up";
	const sign = isUp ? "+" : "-";
	const pct = `${sign}${Math.abs(a.maxPtc).toFixed(2)}%`;
	const duration = formatDuration(a.durationMs);
	const impulseTone = isUp ? "text-emerald-400" : "text-rose-400";

	const endAt = new Date(a.timestamp).getTime();
	const startAt = endAt - a.durationMs;

	useEffect(() => {
		setChartOpen(chartsOpen);
	}, [chartsOpen]);

	function copySymbol() {
		navigator.clipboard.writeText(a.symbol).then(() => {
			setCopied(true);
			setTimeout(() => setCopied(false), 2000);
		});
	}

	return (
		<div className="py-3 pr-4 pl-5">
			<div className="flex items-start justify-between gap-4">
				<div className="flex min-w-0 flex-col gap-0.5">
					<div className="flex items-center gap-2">
						<button
							type="button"
							onClick={copySymbol}
							className="group/sym flex items-center gap-1.5 font-mono text-sm font-semibold tracking-tight transition-colors hover:text-foreground/70"
						>
							{a.symbol}
							<span
								className={cn(
									"text-[10px] transition-all duration-150 font-normal",
									copied
										? "text-emerald-400 opacity-100"
										: "text-muted-foreground/30 opacity-0 group-hover/sym:opacity-100",
								)}
							>
								{copied ? "copied" : null}
							</span>
						</button>
						<span
							className={cn(
								"inline-flex items-center gap-1 font-mono text-[10px] tracking-widest uppercase",
								impulseTone,
							)}
						>
							<span className="text-[9px]">{isUp ? "▲" : "▼"}</span>
							<span>impulse</span>
						</span>
					</div>
					<div className="font-mono text-[11px] text-muted-foreground/50">
						{formatPrice(a.startPrice)}
						<span className="mx-1.5 text-foreground/70">→</span>
						{formatPrice(a.endPrice)}
						<span className="ml-2 opacity-40">{duration}</span>
					</div>
				</div>

				<div className="flex shrink-0 items-center gap-2">
					<span
						className={cn(
							"font-mono text-base font-bold tabular-nums",
							isUp ? "text-emerald-400" : "text-rose-400",
						)}
					>
						{pct}
					</span>
					<button
						type="button"
						onClick={() => setChartOpen((o) => !o)}
						className={cn(
							"text-[10px] text-muted-foreground/30 transition-transform hover:text-muted-foreground/60",
							chartOpen && "rotate-180",
						)}
					>
						▾
					</button>
				</div>
			</div>

			{chartOpen && (
				<div className="mt-2">
					<CandleChart
						symbol={a.symbol}
						startPrice={a.startPrice}
						endPrice={a.endPrice}
						direction={a.direction}
						maxPtc={a.maxPtc}
						startAt={startAt}
						endAt={endAt}
					/>
				</div>
			)}
		</div>
	);
}

function GenericRow({ event }: { event: FeedEvent }) {
	return (
		<div className="py-3 pr-4 pl-5 font-mono text-xs text-muted-foreground">
			<span className="mr-2 tracking-widest uppercase opacity-40">
				{event.type}
			</span>
			{JSON.stringify(event.data)}
		</div>
	);
}

function formatPrice(price: number): string {
	return price >= 1
		? price.toLocaleString("en-US", {
				minimumFractionDigits: 2,
				maximumFractionDigits: 4,
			})
		: price.toFixed(6);
}

function formatDuration(ms: number): string {
	if (ms < 1000) return `${ms}ms`;
	if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
	const m = Math.floor(ms / 60000);
	const s = Math.floor((ms % 60000) / 1000);
	return s > 0 ? `${m}m ${s}s` : `${m}m`;
}
