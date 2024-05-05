import { useTranslation } from "react-i18next";
import { BasicInformationCard } from "./components/Basic.information.card";

function Dashboard() {
  const { t } = useTranslation();

  return (
    <div className="container mx-auto mt-12">
      <div className="text-2xl font-bold mb-6">{t("TITLE")}</div>
      <div className="grid gap-4 md:grid-cols-[1fr_250px] lg:grid-cols-2 lg:gap-8">
        <BasicInformationCard />
        <div>test 1</div>
        <div>test 1</div>
        <div>test 1</div>
      </div>
    </div>
  );
}

export default Dashboard;
