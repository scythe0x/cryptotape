import { cn } from "@/lib/utils";
import type { FeedEvent, WSState, WSStatus } from "@/types";
import { BarChart3, Trash2, Volume2, VolumeX } from "lucide-react";
import type { ComponentProps, ReactNode } from "react";
import { FeedRow } from "./FeedRow";
import { Button } from "./ui/button";

interface Props {
	items: FeedEvent[];
	wsState: WSState;
	muted: boolean;
	minVisiblePct: number;
	chartsOpen: boolean;
	onToggleMute: () => void;
	onToggleCharts: () => void;
	onMinVisiblePctChange: (value: number) => void;
	onClear: () => void;
}

export function Feed({
	items,
	wsState,
	muted,
	minVisiblePct,
	chartsOpen,
	onToggleMute,
	onToggleCharts,
	onMinVisiblePctChange,
	onClear,
}: Props) {
	const { status } = wsState;

	return (
		<div className="flex h-screen flex-col bg-background font-mono">
			<div className="flex items-center justify-between border-b border-border/60 px-4 py-2.5">
				<div className="flex items-center gap-3">
					<span className="text-xs font-semibold tracking-[0.2em] text-foreground/80 uppercase">
						Cryptotape
					</span>
				</div>

				<div className="flex items-center gap-2">
					<label className="flex items-center gap-2 rounded-full border border-border/60 bg-card/50 px-3 py-1.5">
						<span className="text-[10px] tracking-widest text-muted-foreground/60 uppercase">
							min %
						</span>
						<input
							type="number"
							min="0"
							step="0.1"
							value={Number.isFinite(minVisiblePct) ? minVisiblePct : 0}
							onChange={(event) => {
								const next = Number.parseFloat(event.target.value);
								onMinVisiblePctChange(Number.isFinite(next) ? next : 0);
							}}
							className="w-10 border-0 bg-transparent text-[10px] text-foreground/75 outline-none"
						/>
					</label>

					<IconButton
						label="Clear feed"
						onClick={onClear}
						disabled={items.length === 0}
					>
						<Trash2 className="size-3.5" />
					</IconButton>

					<IconButton
						label={chartsOpen ? "Hide charts" : "Show charts"}
						active={chartsOpen}
						activeClassName="border-amber-400/60 text-amber-300"
						onClick={onToggleCharts}
					>
						<BarChart3 className="size-3.5" />
					</IconButton>

					<IconButton
						label={muted ? "Unmute alerts" : "Mute alerts"}
						active={!muted}
						activeClassName="border-amber-400/60 text-amber-300"
						onClick={onToggleMute}
					>
						{muted ? (
							<VolumeX className="size-3.5" />
						) : (
							<Volume2 className="size-3.5" />
						)}
					</IconButton>

					<StatusBadge wsState={wsState} />
				</div>
			</div>

			<div className="flex-1 overflow-y-auto">
				{items.length === 0 ? (
					<div className="flex h-full items-center justify-center">
						<span className="text-xs tracking-widest text-muted-foreground/40 uppercase">
							{status === "connected" ? "waiting for impulses…" : "connecting…"}
						</span>
					</div>
				) : (
					items.map((item) => (
						<FeedRow key={item.id} event={item} chartsOpen={chartsOpen} />
					))
				)}
			</div>

			{items.length > 0 && (
				<div className="border-t border-border/40 px-4 py-1.5 text-[10px] text-muted-foreground/40 tabular-nums">
					{items.length} event{items.length !== 1 ? "s" : ""}
				</div>
			)}
		</div>
	);
}

function StatusBadge({ wsState }: { wsState: WSState }) {
	const { status, pingMs } = wsState;

	const label: Record<WSStatus, string> = {
		connecting: "Connecting to live feed",
		connected: "Connection is live",
		disconnected: "Disconnected from live feed",
	};

	const color: Record<WSStatus, string> = {
		connecting: "bg-amber-400 animate-pulse",
		connected: "bg-emerald-400 shadow-[0_0_12px_rgba(74,222,128,0.55)]",
		disconnected: "bg-rose-500",
	};

	return (
		<Tooltip label={label[status]}>
			<div className="flex items-center gap-2 rounded-full border border-border/60 bg-card/50 px-3 py-1.5">
				<span className={cn("h-1.5 w-1.5 rounded-full", color[status])} />
				<span className="text-[10px] text-foreground/55 tabular-nums">
					{status === "connected" && pingMs !== null ? `${pingMs} ms` : "--"}
				</span>
			</div>
		</Tooltip>
	);
}

function IconButton({
	label,
	children,
	active = false,
	activeClassName,
	...props
}: {
	label: string;
	children: ReactNode;
	active?: boolean;
	activeClassName?: string;
} & ComponentProps<typeof Button>) {
	return (
		<Button
			type="button"
			variant="outline"
			size="xs"
			className={cn(
				"h-7 rounded-full border-border/60 bg-background/70 px-2 shadow-none",
				active && activeClassName,
			)}
			{...props}
		>
			<Tooltip label={label}>{children}</Tooltip>
		</Button>
	);
}

function Tooltip({ label, children }: { label: string; children: ReactNode }) {
	return (
		<span className="group/tooltip relative inline-flex items-center justify-center">
			{children}
			<span className="pointer-events-none absolute top-full left-1/2 z-20 mt-2 -translate-x-1/2 rounded-md border border-border/60 bg-card px-2 py-1 text-[10px] whitespace-nowrap text-foreground opacity-0 shadow-lg transition-opacity duration-150 group-hover/tooltip:opacity-100">
				{label}
			</span>
		</span>
	);
}
