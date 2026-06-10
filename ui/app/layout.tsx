import type { Metadata } from 'next';
import { Inter } from 'next/font/google';
import './globals.css';
import { Sidebar } from '@/components/layout/Sidebar';
import { Header } from '@/components/layout/Header';

const inter = Inter({ subsets: ['latin'] });

export const metadata: Metadata = {
  title: 'Fabric-X Block Explorer',
  description: 'Fabric-X blockchain explorer UI rebuilt around the live fabric-x-block-explorer REST API.',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className={`${inter.className} min-h-screen bg-[#303030] text-[#e8e8e8] antialiased`}>
        <div className="flex min-h-screen overflow-hidden">
          <Sidebar />
          <div className="flex min-h-screen flex-1 flex-col lg:ml-80">
            <Header />
            <main className="flex-1 overflow-y-auto px-6 pb-10 pt-6 lg:px-8">
              {children}
            </main>
          </div>
        </div>
      </body>
    </html>
  );
}
