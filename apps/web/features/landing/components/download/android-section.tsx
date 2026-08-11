"use client";

import Link from "next/link";
// Named import, NOT default — mirrors packages/views/settings/lark-tab.tsx:
// react-qr-code is CJS and the named export maps straight to
// `exports.QRCode`, which survives every bundler's ESM interop.
import { QRCode } from "react-qr-code";
import { useLocale } from "../../i18n";
import type { DownloadAssets } from "../../utils/parse-release-assets";
import { AndroidIcon } from "./os-icons";

interface Props {
  assets: DownloadAssets;
  /** GitHub releases page, shown when no APK asset was resolved. */
  fallbackHref: string;
}

/**
 * Android sideload section. The fork ships the mobile app as a
 * direct-download APK on the same GitHub release as the desktop
 * installers; phones install it by scanning the QR code, so the code
 * encodes the absolute asset URL rather than a page-relative path.
 */
export function AndroidSection({ assets, fallbackHref }: Props) {
  const { t } = useLocale();
  const d = t.download.android;
  const apkUrl = assets.androidApk;

  return (
    <section
      id="android"
      className="bg-[#f7f7f5] py-20 text-[#0a0d12] sm:py-24"
    >
      <div className="mx-auto max-w-[920px] px-4 sm:px-6 lg:px-8">
        <div className="flex flex-col gap-10 md:flex-row md:items-center md:justify-between">
          <div className="max-w-[460px]">
            <div className="flex items-center gap-3">
              <AndroidIcon size={30} className="text-[#0a0d12]" />
              <h2 className="landing-serif text-[2.2rem] leading-[1.1] tracking-[-0.03em] sm:text-[2.6rem]">
                {d.title}
              </h2>
            </div>
            <p className="mt-4 text-body-lg leading-7 text-[#0a0d12]/72">
              {d.sub}
            </p>
            {apkUrl ? (
              <Link
                href={apkUrl}
                className="mt-8 inline-flex items-center rounded-full bg-[#0a0d12] px-6 py-3 text-body font-medium text-white transition-opacity hover:opacity-85"
              >
                {d.downloadApk}
              </Link>
            ) : (
              <p className="mt-8 text-body text-[#0a0d12]/55">
                {d.unavailable}{" "}
                <Link
                  href={fallbackHref}
                  className="underline underline-offset-4"
                >
                  {d.viewAllReleases}
                </Link>
              </p>
            )}
          </div>

          <div className="flex flex-col items-center gap-3">
            <div className="rounded-2xl border border-[#0a0d12]/10 bg-white p-5">
              {apkUrl ? (
                <QRCode
                  value={apkUrl}
                  size={168}
                  bgColor="#ffffff"
                  fgColor="#0a0d12"
                  aria-label={d.scanHint}
                />
              ) : (
                <div className="flex h-[168px] w-[168px] items-center justify-center text-center text-caption text-[#0a0d12]/45">
                  {d.qrUnavailable}
                </div>
              )}
            </div>
            <p className="text-caption text-[#0a0d12]/55">{d.scanHint}</p>
          </div>
        </div>
      </div>
    </section>
  );
}
