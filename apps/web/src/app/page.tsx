import Header from "@/components/header";
import ConverterApp from "@/components/converter-app";
import Footer from "@/components/footer";

export default function Home() {
  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <ConverterApp />
      <Footer />
    </div>
  );
}
