import {
	CANDLE_COLORS,
	CHART_ACCENT_COLORS,
	IMPULSE_CANDLE_COLORS,
} from "@/lib/chart-colors";
import {
	type CandlestickData,
	CandlestickSeries,
	type CandlestickSeriesOptions,
	type IChartApi,
	type IPriceLine,
	type ISeriesApi,
	type Time,
	createChart,
} from "lightweight-charts";
import { useCallback, useEffect, useRef, useState } from "react";

const HISTORY_LIMIT = 500;
const VISIBLE_CANDLES = 100;
const RIGHT_PADDING_BARS = 2;
const BACKGROUND_REFRESH_MS = 30_000;
const MAX_CACHE_AGE_MS = 15_000;
const MAX_SYNTHETIC_GAP_SECONDS = 90;

interface KlineCacheEntry {
	candles: Candle[];
	fetchedAt: number;
}

const KLINE_CACHE = new Map<string, KlineCacheEntry>();
const KLINE_FETCH_PROMISES = new Map<string, Promise<Candle[]>>();

interface Props {
	symbol: string;
	startPrice: number;
	endPrice: number;
	direction: "up" | "down";
	maxPtc: number;
	startAt: number;
	endAt: number;
}

type Candle = CandlestickData<Time>;

interface PriceGuide {
	top: number;
	left: number;
	right: number;
}

async function fetchKlines(
	symbol: string,
	signal: AbortSignal,
): Promise<Candle[]> {
	const res = await fetch(
		`https://fapi.binance.com/fapi/v1/klines?symbol=${symbol}&interval=1m&limit=${HISTORY_LIMIT}`,
		{ signal, cache: "no-store" },
	);
	const raw = (await res.json()) as [
		number,
		string,
		string,
		string,
		string,
		...unknown[],
	][];

	return raw.map((kline) => ({
		time: Math.floor(kline[0] / 1000) as Time,
		open: Number.parseFloat(kline[1]),
		high: Number.parseFloat(kline[2]),
		low: Number.parseFloat(kline[3]),
		close: Number.parseFloat(kline[4]),
	}));
}

function fetchKlinesShared(symbol: string): Promise<Candle[]> {
	const inFlight = KLINE_FETCH_PROMISES.get(symbol);
	if (inFlight) return inFlight;

	const promise = fetch(
		`https://fapi.binance.com/fapi/v1/klines?symbol=${symbol}&interval=1m&limit=${HISTORY_LIMIT}`,
		{ cache: "no-store" },
	)
		.then(async (response) => {
			const raw = (await response.json()) as [
				number,
				string,
				string,
				string,
				string,
				...unknown[],
			][];

			return raw.map((kline) => ({
				time: Math.floor(kline[0] / 1000) as Time,
				open: Number.parseFloat(kline[1]),
				high: Number.parseFloat(kline[2]),
				low: Number.parseFloat(kline[3]),
				close: Number.parseFloat(kline[4]),
			}));
		})
		.finally(() => {
			KLINE_FETCH_PROMISES.delete(symbol);
		});

	KLINE_FETCH_PROMISES.set(symbol, promise);
	return promise;
}

const toMinute = (ms: number): Time => (Math.floor(ms / 60000) * 60) as Time;

function getVisibleLogicalRange(totalCandles: number) {
	const lastLogical = totalCandles - 0.5;
	return {
		from: Math.max(-0.5, totalCandles - VISIBLE_CANDLES - 0.5),
		to: lastLogical + RIGHT_PADDING_BARS,
	};
}

function findCandleIndex(candles: Candle[], time: Time) {
	for (let i = candles.length - 1; i >= 0; i -= 1) {
		if (candles[i].time <= time) return i;
	}

	return -1;
}

function colorizeImpulseRange(candles: Candle[], startAt: Time, endAt: Time) {
	return candles.map((candle) => {
		const inRange = candle.time >= startAt && candle.time <= endAt;
		if (!inRange) return candle;

		return {
			...candle,
			color: IMPULSE_CANDLE_COLORS.up,
			borderColor: IMPULSE_CANDLE_COLORS.up,
			wickColor: IMPULSE_CANDLE_COLORS.wickUp,
		};
	});
}

function createSyntheticCandle(time: Time, open: number): Candle {
	return {
		time,
		open,
		high: open,
		low: open,
		close: open,
	};
}

function buildDisplayCandles(
	baseCandles: Candle[],
	startPrice: number,
	direction: "up" | "down",
	maxPtc: number,
	startAt: number,
	endAt: number,
	endPrice: number,
) {
	if (baseCandles.length === 0) return [];

	const candles = baseCandles.map((candle) => ({ ...candle }));
	const impulseStart = toMinute(startAt);
	const impulseEnd = toMinute(endAt);

	const lastBase = candles[candles.length - 1];
	let cursor = Number(lastBase.time);
	const gapToImpulseEnd = Number(impulseEnd) - cursor;

	while (
		cursor < Number(impulseEnd) &&
		gapToImpulseEnd <= MAX_SYNTHETIC_GAP_SECONDS
	) {
		const previous = candles[candles.length - 1];
		const nextTime = (cursor + 60) as Time;
		candles.push(createSyntheticCandle(nextTime, previous.close));
		cursor = Number(nextTime);
	}

	const activeIndex = findCandleIndex(candles, impulseEnd);
	if (activeIndex !== -1) {
		const active = candles[activeIndex];
		const extremePrice = startPrice * (1 + maxPtc / 100);

		candles[activeIndex] = {
			...active,
			close: endPrice,
			high:
				direction === "up"
					? Math.max(active.high, endPrice, extremePrice)
					: Math.max(active.high, active.open, endPrice),
			low:
				direction === "down"
					? Math.min(active.low, endPrice, extremePrice)
					: Math.min(active.low, active.open, endPrice),
		};
	}

	return colorizeImpulseRange(candles, impulseStart, impulseEnd);
}

function shouldRefreshCandles(
	candles: Candle[],
	endAt: number,
	fetchedAt: number,
) {
	if (candles.length === 0) return true;
	if (Date.now() - fetchedAt > MAX_CACHE_AGE_MS) return true;

	const lastCandleTime = Number(candles[candles.length - 1].time);
	const targetTime = Number(toMinute(endAt));
	return targetTime - lastCandleTime > MAX_SYNTHETIC_GAP_SECONDS;
}

export function CandleChart({
	symbol,
	startPrice,
	endPrice,
	direction,
	maxPtc,
	startAt,
	endAt,
}: Props) {
	const containerRef = useRef<HTMLDivElement>(null);
	const chartRef = useRef<IChartApi | null>(null);
	const seriesRef = useRef<ISeriesApi<"Candlestick"> | null>(null);
	const candlesRef = useRef<Candle[]>([]);
	const displayCandlesRef = useRef<Candle[]>([]);
	const priceLinesRef = useRef<IPriceLine[]>([]);
	const syncGuideRef = useRef<() => void>(() => {});
	const applyDecorationsRef = useRef<() => void>(() => {});
	const refreshKlinesRef = useRef<() => Promise<void>>(async () => {});
	const guideFrameRef = useRef<number | null>(null);
	const autoFollowRef = useRef(true);
	const ignoreRangeChangeRef = useRef(false);
	const fetchInFlightRef = useRef(false);
	const [priceGuide, setPriceGuide] = useState<PriceGuide | null>(null);

	const scheduleGuideSync = useCallback(() => {
		if (guideFrameRef.current !== null) {
			cancelAnimationFrame(guideFrameRef.current);
		}

		guideFrameRef.current = requestAnimationFrame(() => {
			guideFrameRef.current = null;
			syncGuideRef.current();
		});
	}, []);

	const syncPriceGuide = useCallback(() => {
		const chart = chartRef.current;
		const series = seriesRef.current;
		const container = containerRef.current;
		const candles = displayCandlesRef.current;

		if (!chart || !series || !container || candles.length === 0) {
			setPriceGuide(null);
			return;
		}

		const priceY = series.priceToCoordinate(endPrice);
		const guideIndex = findCandleIndex(candles, toMinute(endAt));

		if (priceY === null || guideIndex === -1) {
			setPriceGuide(null);
			return;
		}

		const candleCenterX = chart
			.timeScale()
			.timeToCoordinate(candles[guideIndex].time);
		if (candleCenterX === null) {
			setPriceGuide(null);
			return;
		}

		const nextCandleTime = candles[guideIndex + 1]?.time;
		const nextCandleX =
			nextCandleTime !== undefined
				? chart.timeScale().timeToCoordinate(nextCandleTime)
				: null;
		const candleHalfWidth =
			nextCandleX === null
				? 6
				: Math.max(4, (nextCandleX - candleCenterX) * 0.45);

		const left = candleCenterX + candleHalfWidth;
		const right = Math.max(series.priceScale().width() + 8, 14);
		const maxLeft = container.clientWidth - right - 6;

		if (left >= maxLeft) {
			setPriceGuide(null);
			return;
		}

		setPriceGuide({ top: priceY, left, right });
	}, [endAt, endPrice]);

	const applyDecorations = useCallback(() => {
		const series = seriesRef.current;
		const baseCandles = candlesRef.current;
		if (!series || baseCandles.length === 0) return;

		const displayCandles = buildDisplayCandles(
			baseCandles,
			startPrice,
			direction,
			maxPtc,
			startAt,
			endAt,
			endPrice,
		);
		displayCandlesRef.current = displayCandles;
		series.setData(displayCandles);
		if (autoFollowRef.current) {
			ignoreRangeChangeRef.current = true;
			chartRef.current
				?.timeScale()
				.setVisibleLogicalRange(getVisibleLogicalRange(displayCandles.length));
			requestAnimationFrame(() => {
				ignoreRangeChangeRef.current = false;
			});
		}

		for (const line of priceLinesRef.current) {
			series.removePriceLine(line);
		}

		priceLinesRef.current = [
			series.createPriceLine({
				price: endPrice,
				color: CHART_ACCENT_COLORS.priceGuide,
				lineVisible: false,
				axisLabelVisible: true,
				axisLabelColor: CHART_ACCENT_COLORS.priceLabelBackground,
				axisLabelTextColor: CHART_ACCENT_COLORS.priceLabelText,
				title: "",
			}),
		];

		scheduleGuideSync();
	}, [
		direction,
		endAt,
		endPrice,
		maxPtc,
		scheduleGuideSync,
		startAt,
		startPrice,
	]);

	useEffect(() => {
		syncGuideRef.current = syncPriceGuide;
	}, [syncPriceGuide]);

	useEffect(() => {
		applyDecorationsRef.current = applyDecorations;
	}, [applyDecorations]);

	const refreshKlines = useCallback(async () => {
		if (fetchInFlightRef.current) return;
		fetchInFlightRef.current = true;

		try {
			const candles = await fetchKlinesShared(symbol);
			KLINE_CACHE.set(symbol, {
				candles,
				fetchedAt: Date.now(),
			});
			candlesRef.current = candles;
			displayCandlesRef.current = candles;
			applyDecorationsRef.current();
		} catch (error: unknown) {
			if (!(error instanceof DOMException && error.name === "AbortError")) {
				console.warn("failed to refresh klines", symbol, error);
			}
		} finally {
			fetchInFlightRef.current = false;
		}
	}, [symbol]);

	useEffect(() => {
		refreshKlinesRef.current = refreshKlines;
	}, [refreshKlines]);

	useEffect(() => {
		const container = containerRef.current;
		if (!container) return;

		const chart = createChart(container, {
			width: container.clientWidth,
			height: 320,
			layout: {
				background: { color: "transparent" },
				textColor: "rgba(255,255,255,0.25)",
				fontSize: 10,
			},
			grid: {
				horzLines: { visible: false },
				vertLines: { visible: false },
			},
			crosshair: {
				vertLine: { visible: false },
				horzLine: { visible: false },
			},
			rightPriceScale: {
				borderColor: "rgba(255,255,255,0.06)",
			},
			timeScale: {
				borderColor: "rgba(255,255,255,0.06)",
				timeVisible: true,
				secondsVisible: false,
				barSpacing: 10,
				lockVisibleTimeRangeOnResize: true,
				fixLeftEdge: false,
				fixRightEdge: false,
			},
		});

		const series = chart.addSeries(CandlestickSeries, {
			upColor: CANDLE_COLORS.up,
			downColor: CANDLE_COLORS.down,
			borderUpColor: CANDLE_COLORS.up,
			borderDownColor: CANDLE_COLORS.down,
			wickUpColor: CANDLE_COLORS.wickUp,
			wickDownColor: CANDLE_COLORS.wickDown,
			priceLineVisible: false,
			lastValueVisible: false,
		} as Partial<CandlestickSeriesOptions>);

		chartRef.current = chart;
		seriesRef.current = series;
		autoFollowRef.current = true;

		const resizeObserver = new ResizeObserver(() => {
			chart.applyOptions({ width: container.clientWidth });
			scheduleGuideSync();
		});
		const handleVisibleRangeChange = () => {
			if (!ignoreRangeChangeRef.current) {
				autoFollowRef.current = false;
			}
			scheduleGuideSync();
		};
		const stopAutoFollow = () => {
			autoFollowRef.current = false;
		};

		resizeObserver.observe(container);
		chart
			.timeScale()
			.subscribeVisibleLogicalRangeChange(handleVisibleRangeChange);
		container.addEventListener("wheel", stopAutoFollow, { passive: true });
		container.addEventListener("pointerdown", stopAutoFollow);

		const abortController = new AbortController();

		const cachedEntry = KLINE_CACHE.get(symbol);
		if (cachedEntry) {
			candlesRef.current = cachedEntry.candles;
			displayCandlesRef.current = cachedEntry.candles;
			ignoreRangeChangeRef.current = true;
			chart
				.timeScale()
				.setVisibleLogicalRange(
					getVisibleLogicalRange(cachedEntry.candles.length),
				);
			requestAnimationFrame(() => {
				ignoreRangeChangeRef.current = false;
			});
			applyDecorationsRef.current();
		} else {
			fetchKlines(symbol, abortController.signal)
				.then((candles) => {
					KLINE_CACHE.set(symbol, {
						candles,
						fetchedAt: Date.now(),
					});
					candlesRef.current = candles;
					displayCandlesRef.current = candles;
					ignoreRangeChangeRef.current = true;
					chart
						.timeScale()
						.setVisibleLogicalRange(getVisibleLogicalRange(candles.length));
					requestAnimationFrame(() => {
						ignoreRangeChangeRef.current = false;
					});
					applyDecorationsRef.current();
				})
				.catch((error: unknown) => {
					if (error instanceof DOMException && error.name === "AbortError") {
						return;
					}

					console.warn("failed to load klines", symbol, error);
				});
		}

		return () => {
			abortController.abort();
			resizeObserver.disconnect();
			chart
				.timeScale()
				.unsubscribeVisibleLogicalRangeChange(handleVisibleRangeChange);
			container.removeEventListener("wheel", stopAutoFollow);
			container.removeEventListener("pointerdown", stopAutoFollow);

			if (guideFrameRef.current !== null) {
				cancelAnimationFrame(guideFrameRef.current);
			}

			chart.remove();
			chartRef.current = null;
			seriesRef.current = null;
			candlesRef.current = [];
			displayCandlesRef.current = [];
			priceLinesRef.current = [];
			setPriceGuide(null);
		};
	}, [scheduleGuideSync, symbol]);

	useEffect(() => {
		const cachedEntry = KLINE_CACHE.get(symbol);
		if (!cachedEntry) return;
		if (
			!shouldRefreshCandles(cachedEntry.candles, endAt, cachedEntry.fetchedAt)
		) {
			return;
		}

		void refreshKlinesRef.current();
	}, [endAt, symbol]);

	useEffect(() => {
		if (typeof document === "undefined") return;

		const refreshIfVisible = () => {
			if (document.visibilityState !== "visible") return;
			void refreshKlinesRef.current();
		};

		const interval = window.setInterval(
			refreshIfVisible,
			BACKGROUND_REFRESH_MS,
		);
		document.addEventListener("visibilitychange", refreshIfVisible);

		return () => {
			window.clearInterval(interval);
			document.removeEventListener("visibilitychange", refreshIfVisible);
		};
	}, []);

	useEffect(() => {
		applyDecorations();
	}, [applyDecorations]);

	return (
		<div ref={containerRef} className="relative w-full">
			{priceGuide && (
				<div
					aria-hidden="true"
					className="pointer-events-none absolute h-px rounded-full"
					style={{
						top: `${priceGuide.top}px`,
						left: `${priceGuide.left}px`,
						right: `${priceGuide.right}px`,
						backgroundColor: CHART_ACCENT_COLORS.priceGuide,
						boxShadow: `0 0 12px ${CHART_ACCENT_COLORS.priceGuideGlow}`,
					}}
				/>
			)}
		</div>
	);
}
