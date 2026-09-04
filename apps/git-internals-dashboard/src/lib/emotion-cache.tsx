"use client";

import * as React from "react";
import createCache, { type EmotionCache, type Options as EmotionCacheOptions } from "@emotion/cache";
import { useServerInsertedHTML } from "next/navigation";
import { CacheProvider } from "@emotion/react";

export function createEmotionRegistry(options: EmotionCacheOptions) {
  const cache = createCache(options);
  cache.compat = true;
  const prevInsert = cache.insert;
  let inserted: { name: string; isGlobal: boolean }[] = [];
  cache.insert = (...args) => {
    const [selector, serialized] = args;
    if (cache.inserted[serialized.name] === undefined) {
      inserted.push({ name: serialized.name, isGlobal: !selector });
    }
    return prevInsert(...args);
  };
  const flush = () => {
    const prev = inserted;
    inserted = [];
    return prev;
  };
  return { cache, flush };
}

export function EmotionRegistryProvider({
  registry,
  children,
}: {
  registry: { cache: EmotionCache; flush: () => { name: string; isGlobal: boolean }[] };
  children: React.ReactNode;
}) {
  useServerInsertedHTML(() => {
    const flushed = registry.flush();
    if (flushed.length === 0) return null;

    let styles = "";
    let dataEmotionAttribute = registry.cache.key;
    const globals: { name: string; style: string }[] = [];

    flushed.forEach(({ name, isGlobal }) => {
      const style = registry.cache.inserted[name];
      if (typeof style === "string") {
        if (isGlobal) globals.push({ name, style });
        else {
          dataEmotionAttribute += ` ${name}`;
          styles += style;
        }
      }
    });

    return (
      <>
        {globals.map(({ name, style }) => (
          <style
            key={name}
            data-emotion={`${registry.cache.key}-global ${name}`}
            dangerouslySetInnerHTML={{ __html: style }}
          />
        ))}
        {styles && <style data-emotion={dataEmotionAttribute} dangerouslySetInnerHTML={{ __html: styles }} />}
      </>
    );
  });

  return <CacheProvider value={registry.cache}>{children}</CacheProvider>;
}
